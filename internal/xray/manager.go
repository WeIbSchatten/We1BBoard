package xray

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
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

func (m *Manager) GenerateConfig() (map[string]any, error) {
	var inbounds []model.Inbound
	if err := database.DB.Preload("Clients").Where("enable = ?", true).Find(&inbounds).Error; err != nil {
		return nil, err
	}
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
		xInbounds = append(xInbounds, obj)
	}

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

	routingRules := []map[string]any{
		{"type": "field", "inboundTag": []string{"api"}, "outboundTag": "api"},
	}

	for _, b := range bridges {
		tag := b.OutboundTag
		if tag == "" {
			tag = fmt.Sprintf("bridge-%d", b.ID)
		}
		outbound := buildBridgeOutbound(b, tag)
		xOutbounds = append(xOutbounds, outbound)
		if b.RoutingInbound != "" {
			tags := splitCSV(b.RoutingInbound)
			routingRules = append(routingRules, map[string]any{
				"type":        "field",
				"inboundTag":  tags,
				"outboundTag": tag,
			})
		}
	}

	for _, r := range rules {
		rule := map[string]any{"type": "field", "outboundTag": r.OutboundTag}
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

	cfg := map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
			"access":   filepath.Join(m.ConfigDir, "access.log"),
			"error":    filepath.Join(m.ConfigDir, "error.log"),
		},
		"api": map[string]any{
			"tag":      "api",
			"services": []string{"HandlerService", "LoggerService", "StatsService"},
		},
		"stats": map[string]any{},
		"policy": map[string]any{
			"levels": map[string]any{
				"0": map[string]any{"statsUserUplink": true, "statsUserDownlink": true},
			},
			"system": map[string]any{
				"statsInboundUplink":    true,
				"statsInboundDownlink":  true,
				"statsOutboundUplink":   true,
				"statsOutboundDownlink": true,
			},
		},
		"inbounds":  xInbounds,
		"outbounds": xOutbounds,
		"routing": map[string]any{
			"domainStrategy": routingDomainStrategy(),
			"rules":          routingRules,
		},
	}
	return cfg, nil
}

// ConfigIssue describes an enabled inbound skipped while generating Xray config.
type ConfigIssue struct {
	Tag    string `json:"tag"`
	Reason string `json:"reason"`
}

func routingDomainStrategy() string {
	v := strings.TrimSpace(database.GetSetting("routingDomainStrategy"))
	switch v {
	case "IPIfNonMatch", "IPOnDemand", "AsIs":
		return v
	default:
		return "AsIs"
	}
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
	obj, err := adap.ToXrayInbound(in, in.Clients)
	if err != nil {
		return nil, &ConfigIssue{Tag: tag, Reason: "ToXrayInbound: " + err.Error()}
	}
	return obj, nil
}

// ConfigIssues returns enabled inbounds that would be skipped when generating config.
func (m *Manager) ConfigIssues() ([]ConfigIssue, error) {
	var inbounds []model.Inbound
	if err := database.DB.Preload("Clients").Where("enable = ?", true).Find(&inbounds).Error; err != nil {
		return nil, err
	}
	issues := make([]ConfigIssue, 0)
	for i := range inbounds {
		_, issue := buildXrayInbound(&inbounds[i])
		if issue != nil {
			issues = append(issues, *issue)
		}
	}
	return issues, nil
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
	logW, err := m.openProcessLog()
	if err != nil {
		return fmt.Errorf("open process.log: %w", err)
	}
	m.proc = supervisor.New("xray", m.Bin, "run", "-c", m.ConfigPath())
	m.proc.SetLog(logW)
	return m.proc.Start()
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
