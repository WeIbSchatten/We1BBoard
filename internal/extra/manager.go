package extra

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

// Manager supervises mtg / tuic / hysteria2 processes for non-xray inbounds.
type Manager struct {
	MtgBin    string
	TUICBin   string
	Hy2Bin    string
	ConfigDir string
	mu        sync.Mutex
	procs     map[uint]*supervisor.Process
}

func NewManager(mtg, tuic, hy2, configDir string) *Manager {
	return &Manager{
		MtgBin: mtg, TUICBin: tuic, Hy2Bin: hy2,
		ConfigDir: configDir,
		procs:     map[uint]*supervisor.Process{},
	}
}

func (m *Manager) SyncAll() error {
	var inbounds []model.Inbound
	if err := database.DB.Preload("Clients").Where("enable = ?", true).Find(&inbounds).Error; err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	wanted := map[uint]bool{}
	for i := range inbounds {
		in := &inbounds[i]
		adap, err := protocol.Get(in.Protocol)
		if err != nil || adap.Engine() == "xray" {
			continue
		}
		wanted[in.ID] = true
		if err := m.startLocked(in); err != nil {
			fmt.Printf("[extra] inbound %d: %v\n", in.ID, err)
		}
	}
	for id, proc := range m.procs {
		if !wanted[id] {
			_ = proc.Stop()
			delete(m.procs, id)
		}
	}
	return nil
}

func (m *Manager) startLocked(in *model.Inbound) error {
	if old, ok := m.procs[in.ID]; ok {
		_ = old.Stop()
		delete(m.procs, in.ID)
	}
	_ = os.MkdirAll(m.ConfigDir, 0o755)
	switch in.Protocol {
	case model.ProtoMTProto:
		return m.startMTG(in)
	case model.ProtoTUIC:
		return m.startTUIC(in)
	case model.ProtoHysteria2:
		return m.startHy2(in)
	default:
		return fmt.Errorf("unsupported extra protocol %s", in.Protocol)
	}
}

func (m *Manager) startMTG(in *model.Inbound) error {
	settings := protocol.ParseSettings(in.Settings)
	secret, _ := settings["secret"].(string)
	if secret == "" && len(in.Clients) > 0 {
		secret = in.Clients[0].Password
	}
	if secret == "" {
		return fmt.Errorf("mtproto secret required")
	}
	if _, err := os.Stat(m.MtgBin); err != nil {
		return fmt.Errorf("mtg binary missing: %s", m.MtgBin)
	}
	bind := fmt.Sprintf("%s:%d", in.Listen, in.Port)
	if in.Listen == "" || in.Listen == "0.0.0.0" {
		bind = fmt.Sprintf("0.0.0.0:%d", in.Port)
	}
	proc := supervisor.New(fmt.Sprintf("mtg-%d", in.ID), m.MtgBin, "run", secret, bind)
	if err := proc.Start(); err != nil {
		return err
	}
	m.procs[in.ID] = proc
	return nil
}

func (m *Manager) startTUIC(in *model.Inbound) error {
	if _, err := os.Stat(m.TUICBin); err != nil {
		return fmt.Errorf("tuic binary missing: %s", m.TUICBin)
	}
	users := map[string]string{}
	for _, c := range in.Clients {
		if c.Enable {
			users[c.UUID] = c.Password
		}
	}
	settings := protocol.ParseSettings(in.Settings)
	cfg := map[string]any{
		"server": fmt.Sprintf("%s:%d", nullable(in.Listen, "0.0.0.0"), in.Port),
		"users":  users,
		"certificate": settings["certificate"],
		"private_key": settings["private_key"],
		"congestion_control": "bbr",
		"alpn": []string{"h3"},
	}
	path := filepath.Join(m.ConfigDir, fmt.Sprintf("tuic-%d.json", in.ID))
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return err
	}
	proc := supervisor.New(fmt.Sprintf("tuic-%d", in.ID), m.TUICBin, "-c", path)
	if err := proc.Start(); err != nil {
		return err
	}
	m.procs[in.ID] = proc
	return nil
}

func (m *Manager) startHy2(in *model.Inbound) error {
	if _, err := os.Stat(m.Hy2Bin); err != nil {
		return fmt.Errorf("hysteria binary missing: %s", m.Hy2Bin)
	}
	settings := protocol.ParseSettings(in.Settings)
	auth := map[string]string{}
	for _, c := range in.Clients {
		if c.Enable {
			pw := c.Password
			if pw == "" {
				pw = c.UUID
			}
			auth[c.Email] = pw
		}
	}
	cfg := map[string]any{
		"listen": fmt.Sprintf(":%d", in.Port),
		"tls": map[string]any{
			"cert": settings["cert"],
			"key":  settings["key"],
		},
		"auth": map[string]any{"type": "userpass", "userpass": auth},
	}
	path := filepath.Join(m.ConfigDir, fmt.Sprintf("hy2-%d.yaml", in.ID))
	b, _ := json.MarshalIndent(cfg, "", "  ")
	// write JSON-compatible; hysteria accepts yaml/json-like
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return err
	}
	proc := supervisor.New(fmt.Sprintf("hy2-%d", in.ID), m.Hy2Bin, "server", "-c", path)
	if err := proc.Start(); err != nil {
		return err
	}
	m.procs[in.ID] = proc
	return nil
}

func (m *Manager) Status() []map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]map[string]any, 0, len(m.procs))
	for id, p := range m.procs {
		out = append(out, map[string]any{"inboundId": id, "running": p.IsRunning(), "name": p.Name})
	}
	return out
}

func nullable(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
