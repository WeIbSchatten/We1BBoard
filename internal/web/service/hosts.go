package service

import (
	"fmt"
	"strings"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/protocol"
	"github.com/we1bboard/we1bboard/internal/security"
)

type HostService struct{}

func (s *HostService) List() ([]model.Host, error) {
	var rows []model.Host
	err := database.DB.Order("sort_order asc, id asc").Find(&rows).Error
	return rows, err
}

func (s *HostService) Create(h *model.Host) error {
	if err := validateHost(h); err != nil {
		return err
	}
	return database.DB.Create(h).Error
}

func (s *HostService) Update(h *model.Host) error {
	if h.ID == 0 {
		return fmt.Errorf("id required")
	}
	var existing model.Host
	if err := database.DB.First(&existing, h.ID).Error; err != nil {
		return err
	}
	if err := validateHost(h); err != nil {
		return err
	}
	return database.DB.Save(h).Error
}

func (s *HostService) Delete(id uint) error {
	return database.DB.Delete(&model.Host{}, id).Error
}

func (s *HostService) SetEnable(id uint, enable bool) error {
	res := database.DB.Model(&model.Host{}).Where("id = ?", id).Update("enable", enable)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("host not found")
	}
	return nil
}

func validateHost(h *model.Host) error {
	h.Address = strings.TrimSpace(h.Address)
	h.Remark = strings.TrimSpace(h.Remark)
	h.InboundTag = strings.TrimSpace(h.InboundTag)
	h.SNI = strings.TrimSpace(h.SNI)
	h.HostHeader = strings.TrimSpace(h.HostHeader)
	h.Path = strings.TrimSpace(h.Path)
	h.ALPN = strings.TrimSpace(h.ALPN)
	h.Fingerprint = strings.TrimSpace(h.Fingerprint)
	if h.Address == "" {
		return fmt.Errorf("address required")
	}
	if !security.ValidShareHost(h.Address) {
		return fmt.Errorf("invalid address")
	}
	if h.Port < 0 || h.Port > 65535 {
		return fmt.Errorf("invalid port")
	}
	if h.InboundID > 0 {
		var in model.Inbound
		if err := database.DB.First(&in, h.InboundID).Error; err != nil {
			return fmt.Errorf("inbound not found")
		}
	}
	return nil
}

// EnabledHosts returns all enabled hosts (caller filters by inbound).
func EnabledHosts() ([]model.Host, error) {
	var rows []model.Host
	err := database.DB.Where("enable = ?", true).Order("sort_order asc, id asc").Find(&rows).Error
	return rows, err
}

// BuildShareLinks resolves hosts for an inbound and builds share links.
func BuildShareLinks(in *model.Inbound, c model.Client, defaultHost string) ([]string, error) {
	hosts, err := EnabledHosts()
	if err != nil {
		return nil, err
	}
	return protocol.ShareLinksForHosts(in, c, defaultHost, hosts)
}
