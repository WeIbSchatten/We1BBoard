package service

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/protocol"
	"github.com/we1bboard/we1bboard/internal/sub"
	"github.com/we1bboard/we1bboard/internal/ufw"
	"github.com/we1bboard/we1bboard/internal/web/runtime"
	"github.com/we1bboard/we1bboard/internal/xray"
	"golang.org/x/crypto/bcrypt"
)

type InboundService struct {
	RT *runtime.Hub
}

func (s *InboundService) List() ([]model.Inbound, error) {
	var rows []model.Inbound
	err := database.DB.Preload("Clients").Order("id desc").Find(&rows).Error
	return rows, err
}

func (s *InboundService) Get(id uint) (*model.Inbound, error) {
	var in model.Inbound
	if err := database.DB.Preload("Clients").First(&in, id).Error; err != nil {
		return nil, err
	}
	return &in, nil
}

func (s *InboundService) Create(in *model.Inbound) error {
	adap, err := protocol.Get(in.Protocol)
	if err != nil {
		return err
	}
	if err := adap.Validate(in); err != nil {
		return err
	}
	if adap.Engine() == "xray" {
		if err := xray.ValidateInboundStream(in.StreamSettings); err != nil {
			return err
		}
	}
	if in.Tag == "" {
		in.Tag = fmt.Sprintf("inbound-%s-%d", in.Protocol, in.Port)
	}
	if in.Listen == "" {
		in.Listen = "0.0.0.0"
	}
	if in.Settings == "" {
		in.Settings = "{}"
	}
	if in.StreamSettings == "" {
		in.StreamSettings = `{"network":"tcp","security":"none"}`
	}
	if in.Sniffing == "" {
		in.Sniffing = protocol.MustJSON(protocol.DefaultSniffing())
	}
	if err := database.DB.Create(in).Error; err != nil {
		return err
	}
	ufw.SyncInbound(in, false)
	if err := s.RT.ForNode(in.NodeID).Reload(); err != nil {
		return fmt.Errorf("inbound saved but xray reload failed: %w", err)
	}
	return nil
}

func (s *InboundService) Update(in *model.Inbound) error {
	adap, err := protocol.Get(in.Protocol)
	if err != nil {
		return err
	}
	if err := adap.Validate(in); err != nil {
		return err
	}
	if adap.Engine() == "xray" {
		if err := xray.ValidateInboundStream(in.StreamSettings); err != nil {
			return err
		}
	}
	var old model.Inbound
	if err := database.DB.First(&old, in.ID).Error; err != nil {
		return err
	}
	// Preserve fields the structured form may omit.
	if in.Tag == "" {
		in.Tag = old.Tag
	}
	if in.Sniffing == "" {
		in.Sniffing = old.Sniffing
	}
	if in.NodeID == nil {
		in.NodeID = old.NodeID
	}
	in.Up = old.Up
	in.Down = old.Down
	in.Total = old.Total
	if in.ExpiryTime == 0 {
		in.ExpiryTime = old.ExpiryTime
	}
	in.CreatedAt = old.CreatedAt
	if err := database.DB.Save(in).Error; err != nil {
		return err
	}
	if old.Port != 0 && old.Port != in.Port {
		ufw.DeleteTCP(old.Port)
	}
	ufw.SyncInbound(in, false)
	_ = s.RT.ForNode(in.NodeID).Reload()
	return nil
}

func (s *InboundService) Delete(id uint) error {
	in, err := s.Get(id)
	if err != nil {
		return err
	}
	ufw.SyncInbound(in, true)
	if err := database.DB.Where("inbound_id = ?", id).Delete(&model.Client{}).Error; err != nil {
		return err
	}
	if err := database.DB.Delete(&model.Inbound{}, id).Error; err != nil {
		return err
	}
	_ = s.RT.ForNode(in.NodeID).Reload()
	return nil
}

// DisableInvalid finds enabled Xray inbounds that fail stream validation, sets enable=false, reloads.
func (s *InboundService) DisableInvalid() (int, error) {
	var rows []model.Inbound
	if err := database.DB.Where("enable = ?", true).Find(&rows).Error; err != nil {
		return 0, err
	}
	n := 0
	for i := range rows {
		in := &rows[i]
		adap, err := protocol.Get(in.Protocol)
		if err != nil || adap.Engine() != "xray" {
			continue
		}
		if err := xray.ValidateInboundStream(in.StreamSettings); err == nil {
			continue
		}
		if err := database.DB.Model(in).Update("enable", false).Error; err != nil {
			return n, err
		}
		n++
	}
	if n > 0 {
		_ = s.RT.ReloadLocal()
	}
	return n, nil
}

type ClientService struct {
	RT *runtime.Hub
}

func (s *ClientService) Create(c *model.Client) error {
	if c.InboundID == 0 {
		return fmt.Errorf("inboundId required")
	}
	var in model.Inbound
	if err := database.DB.First(&in, c.InboundID).Error; err != nil {
		return fmt.Errorf("inbound not found")
	}
	switch in.Protocol {
	case model.ProtoWireGuard, model.ProtoAmneziaWG, model.ProtoTunnel, model.ProtoTUN,
		model.ProtoMTProto, model.ProtoTUIC, model.ProtoHysteria2:
		return fmt.Errorf("clients are not supported for protocol %s", in.Protocol)
	}
	if c.UUID == "" {
		c.UUID = uuid.NewString()
	}
	if c.SubID == "" {
		c.SubID = strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	if !sub.ValidSubID(c.SubID) {
		return fmt.Errorf("invalid subId (16-64 alphanumeric/_/-)")
	}
	if c.Email == "" {
		c.Email = c.UUID[:8] + "@we1b"
	}
	// Ensure email is unique enough for xray (email is the client identity key).
	var exists int64
	_ = database.DB.Model(&model.Client{}).Where("email = ?", c.Email).Count(&exists).Error
	if exists > 0 {
		c.Email = fmt.Sprintf("%s-%s", c.Email, c.UUID[:8])
	}
	if c.Password == "" && (in.Protocol == model.ProtoTrojan || in.Protocol == model.ProtoShadowsocks) {
		c.Password = c.UUID
	}
	if err := database.DB.Create(c).Error; err != nil {
		return err
	}
	if err := s.RT.ForNode(in.NodeID).Reload(); err != nil {
		return fmt.Errorf("client saved but xray reload failed: %w", err)
	}
	return nil
}

func (s *ClientService) Update(c *model.Client) error {
	var old model.Client
	if err := database.DB.First(&old, c.ID).Error; err != nil {
		return err
	}
	if c.SubID == "" {
		c.SubID = old.SubID
	}
	if c.SubID != "" && !sub.ValidSubID(c.SubID) {
		return fmt.Errorf("invalid subId (16-64 alphanumeric/_/-)")
	}
	if c.InboundID == 0 {
		c.InboundID = old.InboundID
	}
	c.Up = old.Up
	c.Down = old.Down
	c.CreatedAt = old.CreatedAt
	if err := database.DB.Save(c).Error; err != nil {
		return err
	}
	var in model.Inbound
	_ = database.DB.First(&in, c.InboundID)
	if err := s.RT.ForNode(in.NodeID).Reload(); err != nil {
		return fmt.Errorf("client updated but xray reload failed: %w", err)
	}
	return nil
}

func (s *ClientService) Delete(id uint) error {
	var c model.Client
	if err := database.DB.First(&c, id).Error; err != nil {
		return err
	}
	if err := database.DB.Delete(&c).Error; err != nil {
		return err
	}
	var in model.Inbound
	_ = database.DB.First(&in, c.InboundID)
	_ = s.RT.ForNode(in.NodeID).Reload()
	return nil
}

// ResetTraffic zeroes up/down counters for a client.
func (s *ClientService) ResetTraffic(id uint) error {
	res := database.DB.Model(&model.Client{}).Where("id = ?", id).Updates(map[string]any{"up": 0, "down": 0})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("client not found")
	}
	return nil
}

func (s *ClientService) ShareLink(id uint, host string) (string, error) {
	var c model.Client
	if err := database.DB.First(&c, id).Error; err != nil {
		return "", err
	}
	var in model.Inbound
	if err := database.DB.First(&in, c.InboundID).Error; err != nil {
		return "", err
	}
	adap, err := protocol.Get(in.Protocol)
	if err != nil {
		return "", err
	}
	if host == "" {
		host = database.GetSetting("subHost")
		if host == "" {
			host = "127.0.0.1"
		}
	}
	return adap.ShareLink(&in, c, host)
}

type AuthService struct{}

func (s *AuthService) Login(username, password string) (*model.User, error) {
	var u model.User
	if err := database.DB.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	return &u, nil
}

func (s *AuthService) ChangePassword(username, oldPw, newPw string) error {
	if len(newPw) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	u, err := s.Login(username, oldPw)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return database.DB.Save(u).Error
}

func (s *AuthService) ResetAdmin(username, password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	var u model.User
	if err := database.DB.Where("username = ?", username).First(&u).Error; err != nil {
		u = model.User{Username: username, PasswordHash: string(hash)}
		return database.DB.Create(&u).Error
	}
	u.PasswordHash = string(hash)
	return database.DB.Save(&u).Error
}
