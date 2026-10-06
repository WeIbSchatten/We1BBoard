package tgproxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/supervisor"
)

type Manager struct {
	Bin       string
	ConfigDir string
	mu        sync.Mutex
	procs     map[uint]*supervisor.Process
}

func NewManager(bin, configDir string) *Manager {
	return &Manager{
		Bin:       bin,
		ConfigDir: configDir,
		procs:     map[uint]*supervisor.Process{},
	}
}

func (m *Manager) List() ([]model.TgProxyProfile, error) {
	var rows []model.TgProxyProfile
	err := database.DB.Order("id desc").Find(&rows).Error
	return rows, err
}

func (m *Manager) Get(id uint) (*model.TgProxyProfile, error) {
	var p model.TgProxyProfile
	if err := database.DB.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (m *Manager) Create(p *model.TgProxyProfile) error {
	if p.Name == "" || p.Hostname == "" || p.Secret == "" || p.MTProxyAddr == "" {
		return fmt.Errorf("name, hostname, secret, mtproxyAddr required")
	}
	if p.CarrierMode == "" {
		p.CarrierMode = "websocket"
	}
	if p.Listen == "" {
		p.Listen = "127.0.0.1:8080"
	}
	return database.DB.Create(p).Error
}

func (m *Manager) Update(p *model.TgProxyProfile) error {
	return database.DB.Save(p).Error
}

func (m *Manager) Delete(id uint) error {
	_ = m.Stop(id)
	return database.DB.Delete(&model.TgProxyProfile{}, id).Error
}

func (m *Manager) WriteConfig(p *model.TgProxyProfile) (string, error) {
	if err := os.MkdirAll(m.ConfigDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(m.ConfigDir, fmt.Sprintf("tgproxy-%d.json", p.ID))
	cfg := map[string]any{
		"listen":          p.Listen,
		"hostname":        p.Hostname,
		"mtproxy_address": p.MTProxyAddr,
		"secret":          p.Secret,
		"carrier":         p.CarrierMode,
		"public_mode":     p.PublicMode,
		"public_dir":      p.PublicSiteDir,
		"app_upstream":    p.AppUpstream,
		"profiles": []map[string]any{
			{
				"hostname": p.Hostname,
				"secret":   p.Secret,
				"carrier":  p.CarrierMode,
			},
		},
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	p.ConfigPath = path
	_ = database.DB.Model(p).Update("config_path", path)
	return path, nil
}

func (m *Manager) Start(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, err := m.Get(id)
	if err != nil {
		return err
	}
	if !p.Enable {
		return fmt.Errorf("profile disabled")
	}
	path, err := m.WriteConfig(p)
	if err != nil {
		return err
	}
	if _, err := os.Stat(m.Bin); err != nil {
		return fmt.Errorf("tproxy-server binary missing at %s", m.Bin)
	}
	if old, ok := m.procs[id]; ok {
		_ = old.Stop()
	}
	proc := supervisor.New(fmt.Sprintf("tgproxy-%d", id), m.Bin, "-config", path)
	if err := proc.Start(); err != nil {
		return err
	}
	m.procs[id] = proc
	return nil
}

func (m *Manager) Stop(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if proc, ok := m.procs[id]; ok {
		err := proc.Stop()
		delete(m.procs, id)
		return err
	}
	return nil
}

func (m *Manager) Status(id uint) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	running := false
	if proc, ok := m.procs[id]; ok {
		running = proc.IsRunning()
	}
	return map[string]any{"id": id, "running": running}
}

func (m *Manager) StartEnabled() {
	var rows []model.TgProxyProfile
	_ = database.DB.Where("enable = ?", true).Find(&rows)
	for _, p := range rows {
		_ = m.Start(p.ID)
	}
}
