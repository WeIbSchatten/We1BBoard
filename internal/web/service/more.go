package service

import (
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/web/runtime"
)

type OutboundService struct {
	RT *runtime.Hub
}

func (s *OutboundService) List() ([]model.Outbound, error) {
	var rows []model.Outbound
	err := database.DB.Order("id asc").Find(&rows).Error
	return rows, err
}

func (s *OutboundService) Create(o *model.Outbound) error {
	if o.Settings == "" {
		o.Settings = "{}"
	}
	if err := database.DB.Create(o).Error; err != nil {
		return err
	}
	return s.RT.ReloadLocal()
}

func (s *OutboundService) Update(o *model.Outbound) error {
	if err := database.DB.Save(o).Error; err != nil {
		return err
	}
	return s.RT.ReloadLocal()
}

func (s *OutboundService) Delete(id uint) error {
	if err := database.DB.Delete(&model.Outbound{}, id).Error; err != nil {
		return err
	}
	return s.RT.ReloadLocal()
}

type NodeService struct{}

func (s *NodeService) List() ([]model.Node, error) {
	var rows []model.Node
	err := database.DB.Order("id desc").Find(&rows).Error
	return rows, err
}

func (s *NodeService) Create(n *model.Node) error {
	if n.TLSMode == "" {
		n.TLSMode = "skip"
	}
	return database.DB.Create(n).Error
}

func (s *NodeService) Update(n *model.Node) error {
	return database.DB.Save(n).Error
}

func (s *NodeService) Delete(id uint) error {
	return database.DB.Delete(&model.Node{}, id).Error
}

type RoutingService struct {
	RT *runtime.Hub
}

func (s *RoutingService) List() ([]model.RoutingRule, error) {
	var rows []model.RoutingRule
	err := database.DB.Order("priority asc, id asc").Find(&rows).Error
	return rows, err
}

func (s *RoutingService) Create(r *model.RoutingRule) error {
	if err := database.DB.Create(r).Error; err != nil {
		return err
	}
	return s.RT.ReloadLocal()
}

func (s *RoutingService) Update(r *model.RoutingRule) error {
	if err := database.DB.Save(r).Error; err != nil {
		return err
	}
	return s.RT.ReloadLocal()
}

func (s *RoutingService) Delete(id uint) error {
	if err := database.DB.Delete(&model.RoutingRule{}, id).Error; err != nil {
		return err
	}
	return s.RT.ReloadLocal()
}
