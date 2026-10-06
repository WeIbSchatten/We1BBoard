package xray

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/protocol"
	"github.com/we1bboard/we1bboard/internal/supervisor"
)

type Manager struct {
	Bin        string
	ConfigDir  string
	APIPort    int
	proc       *supervisor.Process
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
		if in.Tag == "" {
			in.Tag = fmt.Sprintf("inbound-%d", in.ID)
		}
		adap, err := protocol.Get(in.Protocol)
		if err != nil || adap.Engine() != "xray" {
			continue
		}
		obj, err := adap.ToXrayInbound(in, in.Clients)
		if err != nil {
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
		"log": map[string]any{"loglevel": "warning"},
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
			"domainStrategy": "AsIs",
			"rules":          routingRules,
		},
	}
	return cfg, nil
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

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.WriteConfig(); err != nil {
		return err
	}
	if _, err := os.Stat(m.Bin); err != nil {
		return fmt.Errorf("xray binary missing (%s): place xray in bin dir or set WE1B_XRAY_BIN", m.Bin)
	}
	m.proc = supervisor.New("xray", m.Bin, "run", "-c", m.ConfigPath())
	return m.proc.Start()
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.proc == nil {
		return nil
	}
	return m.proc.Stop()
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
