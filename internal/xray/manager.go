package xray

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/panellog"
	"github.com/we1bboard/we1bboard/internal/protocol"
	"github.com/we1bboard/we1bboard/internal/supervisor"
)

const processLogMaxBytes = 8 << 20 // 8 MiB

type Manager struct {
	Bin        string
	ConfigDir  string
	APIPort    int
	proc       *supervisor.Process
	procLog    *os.File
	mu         sync.Mutex
	configPath string
}

func NewManager(bin, configDir string) *Manager {
	return &Manager{
		Bin:       bin,
		ConfigDir: configDir,
		APIPort:   10085,
	}
}

func (m *Manager) ConfigPath() string {
	return filepath.Join(m.ConfigDir, "config.json")
}

func (m *Manager) IsRunning() bool {
	return m.proc != nil && m.proc.IsRunning()
}

// Uptime returns how long the xray process has been running, or 0 if stopped.
func (m *Manager) Uptime() time.Duration {
	if m.proc == nil {
		return 0
	}
	return m.proc.Uptime()
}

func (m *Manager) GenerateConfig() (map[string]any, error) {
	var inbounds []model.Inbound
	if err := database.DB.Where("enable = ?", true).Find(&inbounds).Error; err != nil {
		return nil, err
	}
	attachClientsForConfig(inbounds)
	var outbounds []model.Outbound
	if err := database.DB.Where("enable = ?", true).Find(&outbounds).Error; err != nil {
		return nil, err
	}
	var bridges []model.Bridge
	if err := database.DB.Where("enable = ?", true).Find(&bridges).Error; err != nil {
		return nil, err
	}
	var rules []model.RoutingRule
	if err := database.DB.Where("enable = ?", true).Order("priority asc").Find(&rules).Error; err != nil {
		return nil, err
	}

	cfg := deepCopyMap(LoadTemplate())

	apiInbound := map[string]any{
		"tag":      "api",
		"listen":   "127.0.0.1",
		"port":     m.APIPort,
		"protocol": "dokodemo-door",
		"settings": map[string]any{"address": "127.0.0.1"},
	}

	xInbounds := []map[string]any{apiInbound}
	for i := range inbounds {
		in := &inbounds[i]
		obj, issue := buildXrayInbound(in)
		if issue != nil {
			log.Printf("[xray] skip inbound id=%d tag=%s: %s", in.ID, issue.Tag, issue.Reason)
			continue
		}
		if obj != nil {
			xInbounds = append(xInbounds, obj)
		}
	}
	cfg["inbounds"] = xInbounds

	xOutbounds := make([]map[string]any, 0, len(outbounds)+len(bridges)+2)
	for _, o := range outbounds {
		settings := protocol.ParseSettings(o.Settings)
		stream := protocol.ParseStream(o.StreamSettings)
		item := map[string]any{
			"tag":      o.Tag,
			"protocol": o.Protocol,
			"settings": settings,
		}
		if o.Protocol != "freedom" && o.Protocol != "blackhole" && o.Protocol != "dns" {
			item["streamSettings"] = stream
		}
		xOutbounds = append(xOutbounds, item)
	}

	bridgeRules := make([]map[string]any, 0)
	for _, b := range bridges {
		tag := b.OutboundTag
		if tag == "" {
			tag = fmt.Sprintf("bridge-%d", b.ID)
		}
		xOutbounds = append(xOutbounds, buildBridgeOutbound(b, tag))
		if b.RoutingInbound != "" {
			bridgeRules = append(bridgeRules, map[string]any{
				"type":        "field",
				"inboundTag":  splitCSV(b.RoutingInbound),
				"outboundTag": tag,
			})
		}
	}

	hasAPIOut := false
	for _, o := range xOutbounds {
		if o["tag"] == "api" {
			hasAPIOut = true
			break
		}
	}
	if !hasAPIOut {
		xOutbounds = append([]map[string]any{{
			"tag": "api", "protocol": "freedom", "settings": map[string]any{},
		}}, xOutbounds...)
	}
	cfg["outbounds"] = xOutbounds

	if _, ok := cfg["api"]; !ok {
		cfg["api"] = deepCopyMap(DefaultTemplate())["api"]
	}

	ensureLogPaths(cfg, m.ConfigDir)
	cfg["routing"] = mergeRouting(cfg["routing"], bridgeRules, rules)

	return cfg, nil
}

func ensureLogPaths(cfg map[string]any, configDir string) {
	logObj, _ := cfg["log"].(map[string]any)
	if logObj == nil {
		logObj = map[string]any{"loglevel": "warning"}
		cfg["log"] = logObj
	}
	logObj["access"] = filepath.Join(configDir, "access.log")
	// Write errors BOTH to file and keep visibility: xray only supports one sink.
	// Prefer error.log for persistence; Start() also tails it into panellog/process failures.
	logObj["error"] = filepath.Join(configDir, "error.log")
	if _, ok := logObj["loglevel"]; !ok {
		logObj["loglevel"] = "warning"
	}
}

// TestConfig runs `xray run -test -c config.json` and returns combined output on failure.
func (m *Manager) TestConfig() error {
	if _, err := os.Stat(m.Bin); err != nil {
		return fmt.Errorf("xray binary missing (%s)", m.Bin)
	}
	cmd := exec.Command(m.Bin, "run", "-test", "-c", m.ConfigPath())
	cmd.Dir = m.ConfigDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("xray config test failed:\n%s", msg)
	}
	return nil
}

func mergeRouting(raw any, bridgeRules []map[string]any, dbRules []model.RoutingRule) map[string]any {
	tplRouting, _ := raw.(map[string]any)
	if tplRouting == nil {
		tplRouting = map[string]any{}
	}

	domainStrategy := "AsIs"
	if ds, ok := tplRouting["domainStrategy"].(string); ok && strings.TrimSpace(ds) != "" {
		domainStrategy = ds
	}
	if setting := strings.TrimSpace(database.GetSetting("routingDomainStrategy")); setting != "" {
		switch setting {
		case "IPIfNonMatch", "IPOnDemand", "AsIs":
			domainStrategy = setting
		}
	}

	var tplRules []map[string]any
	if rawRules, ok := tplRouting["rules"].([]any); ok {
		for _, r := range rawRules {
			if m, ok := r.(map[string]any); ok {
				tplRules = append(tplRules, m)
			}
		}
	}

	routingRules := []map[string]any{
		{"type": "field", "inboundTag": []string{"api"}, "outboundTag": "api"},
	}
	routingRules = append(routingRules, bridgeRules...)

	for _, r := range dbRules {
		rule := map[string]any{"type": "field"}
		if strings.TrimSpace(r.BalancerTag) != "" {
			rule["balancerTag"] = r.BalancerTag
		} else {
			rule["outboundTag"] = r.OutboundTag
		}
		if r.InboundTag != "" {
			rule["inboundTag"] = []string{r.InboundTag}
		}
		if r.Domain != "" {
			rule["domain"] = splitCSV(r.Domain)
		}
		if r.IP != "" {
			rule["ip"] = splitCSV(r.IP)
		}
		if r.Port != "" {
			rule["port"] = r.Port
		}
		if r.Network != "" {
			rule["network"] = r.Network
		}
		if r.Protocol != "" {
			rule["protocol"] = splitCSV(r.Protocol)
		}
		routingRules = append(routingRules, rule)
	}

	for _, r := range tplRules {
		if isAPIRule(r) {
			continue
		}
		routingRules = append(routingRules, r)
	}

	out := map[string]any{
		"domainStrategy": domainStrategy,
		"rules":          routingRules,
	}
	if balancers, ok := tplRouting["balancers"]; ok && balancers != nil {
		out["balancers"] = balancers
	}
	return out
}

// ConfigIssue describes an enabled inbound skipped while generating Xray config.
type ConfigIssue struct {
	Tag    string `json:"tag"`
	Reason string `json:"reason"`
}

func inboundTag(in *model.Inbound) string {
	if in.Tag != "" {
		return in.Tag
	}
	return fmt.Sprintf("inbound-%d", in.ID)
}

func buildXrayInbound(in *model.Inbound) (map[string]any, *ConfigIssue) {
	tag := inboundTag(in)
	if in.Tag == "" {
		in.Tag = tag
	}
	adap, err := protocol.Get(in.Protocol)
	if err != nil || adap.Engine() != "xray" {
		return nil, nil // not an xray inbound — not an issue
	}
	stream := protocol.ParseStream(in.StreamSettings)
	if err := ValidateStreamSettings(stream); err != nil {
		return nil, &ConfigIssue{Tag: tag, Reason: err.Error()}
	}
	// Keep DB stream intact for share links; only sanitize what Xray receives.
	sanitized := *in
	sanitized.StreamSettings = protocol.SanitizeInboundStreamJSON(in.StreamSettings)
	obj, err := adap.ToXrayInbound(&sanitized, in.Clients)
	if err != nil {
		return nil, &ConfigIssue{Tag: tag, Reason: "ToXrayInbound: " + err.Error()}
	}
	// Also strip client-only fields from VLESS/etc client maps (limitIp is panel-only).
	if obj != nil {
		stripPanelOnlyClientFields(obj)
	}
	return obj, nil
}

func stripPanelOnlyClientFields(obj map[string]any) {
	settings, _ := obj["settings"].(map[string]any)
	if settings == nil {
		return
	}
	if clients, ok := settings["clients"].([]map[string]any); ok {
		for _, c := range clients {
			delete(c, "limitIp")
			delete(c, "limitip")
		}
		return
	}
	if raw, ok := settings["clients"].([]any); ok {
		for _, item := range raw {
			if c, ok := item.(map[string]any); ok {
				delete(c, "limitIp")
				delete(c, "limitip")
			}
		}
	}
}

// ConfigIssues returns enabled inbounds that would be skipped when generating config.
func (m *Manager) ConfigIssues() ([]ConfigIssue, error) {
	var inbounds []model.Inbound
	if err := database.DB.Where("enable = ?", true).Find(&inbounds).Error; err != nil {
		return nil, err
	}
	attachClientsForConfig(inbounds)
	issues := make([]ConfigIssue, 0)
	for i := range inbounds {
		_, issue := buildXrayInbound(&inbounds[i])
		if issue != nil {
			issues = append(issues, *issue)
		}
	}
	return issues, nil
}

// attachClientsForConfig sets Clients on each inbound to those with InboundID==id OR inboundIds containing id.
func attachClientsForConfig(inbounds []model.Inbound) {
	if len(inbounds) == 0 {
		return
	}
	var clients []model.Client
	if err := database.DB.Find(&clients).Error; err != nil {
		return
	}
	byID := map[uint][]model.Client{}
	for _, c := range clients {
		for _, iid := range model.ParseInboundIDList(&c) {
			byID[iid] = append(byID[iid], c)
		}
	}
	for i := range inbounds {
		inbounds[i].Clients = byID[inbounds[i].ID]
	}
}

func splitCSV(s string) []string {
	parts := []string{}
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				parts = append(parts, cur)
			}
			cur = ""
			continue
		}
		if r != ' ' {
			cur += string(r)
		}
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}

func (m *Manager) WriteConfig() error {
	cfg, err := m.GenerateConfig()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.ConfigDir, 0o755); err != nil {
		return err
	}
	path := m.ConfigPath()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return err
	}
	m.configPath = path
	return nil
}

func (m *Manager) ProcessLogPath() string {
	return filepath.Join(m.ConfigDir, "process.log")
}

func (m *Manager) AccessLogPath() string {
	return filepath.Join(m.ConfigDir, "access.log")
}

func (m *Manager) ErrorLogPath() string {
	return filepath.Join(m.ConfigDir, "error.log")
}

func (m *Manager) openProcessLog() (io.Writer, error) {
	if err := os.MkdirAll(m.ConfigDir, 0o755); err != nil {
		return nil, err
	}
	path := m.ProcessLogPath()
	if info, err := os.Stat(path); err == nil && info.Size() > processLogMaxBytes {
		_ = os.Truncate(path, 0)
	}
	if m.procLog != nil {
		_ = m.procLog.Close()
		m.procLog = nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	m.procLog = f
	return io.MultiWriter(os.Stdout, f), nil
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.WriteConfig(); err != nil {
		return err
	}
	if _, err := os.Stat(m.Bin); err != nil {
		return fmt.Errorf("xray binary missing (%s): place xray in bin dir or set WE1B_XRAY_BIN", m.Bin)
	}
	if err := m.TestConfig(); err != nil {
		panellog.Append("%v", err)
		return err
	}
	logW, err := m.openProcessLog()
	if err != nil {
		return fmt.Errorf("open process.log: %w", err)
	}
	// Stop previous instance fully before binding ports again.
	if m.proc != nil {
		_ = m.proc.Stop()
		m.proc = nil
	}
	m.proc = supervisor.New("xray", m.Bin, "run", "-c", m.ConfigPath())
	m.proc.SetLog(logW)
	m.proc.OnExit = func(err error) {
		tail := m.tailLogFile(m.ErrorLogPath(), 30)
		if err != nil {
			if tail != "" {
				panellog.Append("xray exited: %v\n--- error.log ---\n%s", err, tail)
			} else {
				panellog.Append("xray exited: %v — see Logs → process / error", err)
			}
		} else {
			panellog.Append("xray exited cleanly")
		}
	}
	if err := m.proc.StartAndWaitHealthy(1500 * time.Millisecond); err != nil {
		procTail := m.tailLogFile(m.ProcessLogPath(), 40)
		errTail := m.tailLogFile(m.ErrorLogPath(), 40)
		_ = m.proc.Stop()
		var b strings.Builder
		b.WriteString(err.Error())
		if errTail != "" {
			b.WriteString("\n--- error.log ---\n")
			b.WriteString(errTail)
		}
		if procTail != "" {
			b.WriteString("\n--- process.log ---\n")
			b.WriteString(procTail)
		}
		msg := b.String()
		panellog.Append("xray start failed:\n%s", msg)
		return fmt.Errorf("%s", msg)
	}
	panellog.Append("xray started")
	return nil
}

func (m *Manager) tailLogFile(path string, maxLines int) string {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (m *Manager) tailProcessLog(maxLines int) string {
	return m.tailLogFile(m.ProcessLogPath(), maxLines)
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.proc == nil {
		return nil
	}
	err := m.proc.Stop()
	if m.procLog != nil {
		_ = m.procLog.Close()
		m.procLog = nil
	}
	return err
}

func (m *Manager) Restart() error {
	_ = m.Stop()
	return m.Start()
}

func (m *Manager) Reload() error {
	if err := m.WriteConfig(); err != nil {
		return err
	}
	if !m.IsRunning() {
		return m.Start()
	}
	return m.Restart()
}
