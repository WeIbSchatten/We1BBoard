package service

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

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
	if err := database.DB.Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	attachClientsByMembership(rows)
	return rows, nil
}

func (s *InboundService) Get(id uint) (*model.Inbound, error) {
	var in model.Inbound
	if err := database.DB.First(&in, id).Error; err != nil {
		return nil, err
	}
	rows := []model.Inbound{in}
	attachClientsByMembership(rows)
	in = rows[0]
	return &in, nil
}

// attachClientsByMembership fills Clients for each inbound where InboundID==id or inboundIds contains id.
func attachClientsByMembership(inbounds []model.Inbound) {
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

	var clients []model.Client
	_ = database.DB.Find(&clients).Error
	for _, c := range clients {
		ids := model.ParseInboundIDList(&c)
		belongs := false
		remaining := make([]uint, 0, len(ids))
		for _, iid := range ids {
			if iid == id {
				belongs = true
				continue
			}
			remaining = append(remaining, iid)
		}
		if !belongs {
			continue
		}
		if len(remaining) == 0 {
			_ = database.DB.Delete(&model.Client{}, c.ID).Error
			continue
		}
		model.NormalizeInboundIDs(&c, remaining)
		_ = database.DB.Model(&c).Updates(map[string]any{
			"inbound_id":  c.InboundID,
			"inbound_ids": c.InboundIDs,
		}).Error
	}

	if err := database.DB.Delete(&model.Inbound{}, id).Error; err != nil {
		return err
	}
	_ = s.RT.ForNode(in.NodeID).Reload()
	return nil
}

// Clone copies an inbound without clients, assigns a free random port, and appends " (copy)" to remark.
func (s *InboundService) Clone(id uint) (*model.Inbound, error) {
	src, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	port, err := freeInboundPort(src.Port)
	if err != nil {
		return nil, err
	}
	remark := strings.TrimSpace(src.Remark)
	if remark == "" {
		remark = src.Tag
	}
	if remark == "" {
		remark = string(src.Protocol)
	}
	if !strings.HasSuffix(remark, " (copy)") {
		remark = remark + " (copy)"
	}
	tag := fmt.Sprintf("inbound-%s-%d", src.Protocol, port)
	clone := &model.Inbound{
		Remark:         remark,
		Enable:         src.Enable,
		Listen:         src.Listen,
		Port:           port,
		Protocol:       src.Protocol,
		Settings:       src.Settings,
		StreamSettings: src.StreamSettings,
		Sniffing:       src.Sniffing,
		Tag:            tag,
		NodeID:         src.NodeID,
		Total:          src.Total,
		ExpiryTime:     src.ExpiryTime,
	}
	if err := s.Create(clone); err != nil {
		return nil, err
	}
	return clone, nil
}

func freeInboundPort(prefer int) (int, error) {
	used := map[int]bool{}
	var ports []int
	_ = database.DB.Model(&model.Inbound{}).Pluck("port", &ports).Error
	for _, p := range ports {
		used[p] = true
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for try := 0; try < 200; try++ {
		p := prefer
		if try > 0 || used[p] {
			p = 10000 + rng.Intn(50000)
		}
		if p < 1 || p > 65535 || used[p] {
			continue
		}
		return p, nil
	}
	return 0, fmt.Errorf("could not find a free inbound port")
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

func clientProtocolsOK(proto model.Protocol) error {
	switch proto {
	case model.ProtoWireGuard, model.ProtoAmneziaWG, model.ProtoTunnel, model.ProtoTUN,
		model.ProtoMTProto, model.ProtoTUIC, model.ProtoHysteria2:
		return fmt.Errorf("clients are not supported for protocol %s", proto)
	}
	return nil
}

func (s *ClientService) resolveInboundIDs(c *model.Client) error {
	ids := model.ParseInboundIDsCSV(c.InboundIDs)
	if len(ids) == 0 && c.InboundID != 0 {
		ids = []uint{c.InboundID}
	}
	if len(ids) == 0 {
		return fmt.Errorf("inboundId required")
	}
	for _, id := range ids {
		var in model.Inbound
		if err := database.DB.First(&in, id).Error; err != nil {
			return fmt.Errorf("inbound %d not found", id)
		}
		if err := clientProtocolsOK(in.Protocol); err != nil {
			return err
		}
	}
	model.NormalizeInboundIDs(c, ids)
	c.TrafficReset = model.NormalizeTrafficReset(c.TrafficReset)
	return nil
}

func (s *ClientService) reloadForClient(c *model.Client) error {
	seen := map[uint]bool{}
	var lastErr error
	for _, id := range model.ParseInboundIDList(c) {
		var in model.Inbound
		if err := database.DB.First(&in, id).Error; err != nil {
			continue
		}
		key := uint(0)
		if in.NodeID != nil {
			key = *in.NodeID
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if err := s.RT.ForNode(in.NodeID).Reload(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (s *ClientService) Create(c *model.Client) error {
	if err := s.resolveInboundIDs(c); err != nil {
		return err
	}
	var in model.Inbound
	if err := database.DB.First(&in, c.InboundID).Error; err != nil {
		return fmt.Errorf("inbound not found")
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
	if err := s.reloadForClient(c); err != nil {
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
	if strings.TrimSpace(c.InboundIDs) == "" && c.InboundID == 0 {
		c.InboundID = old.InboundID
		c.InboundIDs = old.InboundIDs
	}
	if err := s.resolveInboundIDs(c); err != nil {
		return err
	}
	c.Up = old.Up
	c.Down = old.Down
	c.CreatedAt = old.CreatedAt
	if err := database.DB.Save(c).Error; err != nil {
		return err
	}
	if err := s.reloadForClient(c); err != nil {
		return fmt.Errorf("client updated but xray reload failed: %w", err)
	}
	_ = s.reloadForClient(&old) // also reload previous inbounds if detached
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
	_ = s.reloadForClient(&c)
	return nil
}

// ResetTraffic zeroes up/down counters for a client.
func (s *ClientService) ResetTraffic(id uint) error {
	res := database.DB.Model(&model.Client{}).Where("id = ?", id).Updates(map[string]any{
		"up":                 0,
		"down":               0,
		"last_traffic_reset": time.Now().UnixMilli(),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("client not found")
	}
	return nil
}

// BulkAdjust adds days to expiry and/or GB to totalGB for selected clients.
func (s *ClientService) BulkAdjust(ids []uint, addDays int, addGB int64) (int, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("ids required")
	}
	var clients []model.Client
	if err := database.DB.Where("id IN ?", ids).Find(&clients).Error; err != nil {
		return 0, err
	}
	now := time.Now().UnixMilli()
	n := 0
	for i := range clients {
		c := &clients[i]
		updates := map[string]any{}
		if addDays != 0 {
			base := c.ExpiryTime
			if base <= 0 || base < now {
				base = now
			}
			updates["expiry_time"] = base + int64(addDays)*86400000
		}
		if addGB != 0 {
			next := c.TotalGB + addGB
			if next < 0 {
				next = 0
			}
			updates["total_gb"] = next
		}
		if len(updates) == 0 {
			continue
		}
		if err := database.DB.Model(c).Updates(updates).Error; err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// BulkAttach sets inboundIds (csv membership) for selected clients.
func (s *ClientService) BulkAttach(ids []uint, inboundIDs []uint) (int, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("ids required")
	}
	if len(inboundIDs) == 0 {
		return 0, fmt.Errorf("inboundIds required")
	}
	for _, iid := range inboundIDs {
		var in model.Inbound
		if err := database.DB.First(&in, iid).Error; err != nil {
			return 0, fmt.Errorf("inbound %d not found", iid)
		}
		if err := clientProtocolsOK(in.Protocol); err != nil {
			return 0, err
		}
	}
	var clients []model.Client
	if err := database.DB.Where("id IN ?", ids).Find(&clients).Error; err != nil {
		return 0, err
	}
	n := 0
	for i := range clients {
		c := &clients[i]
		model.NormalizeInboundIDs(c, inboundIDs)
		if err := database.DB.Model(c).Updates(map[string]any{
			"inbound_id":  c.InboundID,
			"inbound_ids": c.InboundIDs,
		}).Error; err != nil {
			return n, err
		}
		_ = s.reloadForClient(c)
		n++
	}
	return n, nil
}

// BulkDetach removes inbound IDs from each client's membership csv.
// If the primary inbound was removed, primary becomes the first remaining.
// Clients that would end with zero inbounds are skipped.
func (s *ClientService) BulkDetach(ids []uint, inboundIDs []uint) (int, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("ids required")
	}
	if len(inboundIDs) == 0 {
		return 0, fmt.Errorf("inboundIds required")
	}
	remove := map[uint]bool{}
	for _, id := range inboundIDs {
		if id > 0 {
			remove[id] = true
		}
	}
	if len(remove) == 0 {
		return 0, fmt.Errorf("inboundIds required")
	}
	var clients []model.Client
	if err := database.DB.Where("id IN ?", ids).Find(&clients).Error; err != nil {
		return 0, err
	}
	n := 0
	for i := range clients {
		c := &clients[i]
		current := model.ParseInboundIDList(c)
		remaining := make([]uint, 0, len(current))
		changed := false
		for _, id := range current {
			if remove[id] {
				changed = true
				continue
			}
			remaining = append(remaining, id)
		}
		if !changed || len(remaining) == 0 {
			continue
		}
		model.NormalizeInboundIDs(c, remaining)
		if err := database.DB.Model(c).Updates(map[string]any{
			"inbound_id":  c.InboundID,
			"inbound_ids": c.InboundIDs,
		}).Error; err != nil {
			return n, err
		}
		_ = s.reloadForClient(c)
		n++
	}
	return n, nil
}

func (s *ClientService) ShareLink(id uint, host string) (string, error) {
	links, err := s.ShareLinks(id, host)
	if err != nil {
		return "", err
	}
	return strings.Join(links, "\n"), nil
}

// ShareLinks returns one link per subscription host (or a single default link).
func (s *ClientService) ShareLinks(id uint, host string) ([]string, error) {
	var c model.Client
	if err := database.DB.First(&c, id).Error; err != nil {
		return nil, err
	}
	var in model.Inbound
	if err := database.DB.First(&in, c.InboundID).Error; err != nil {
		return nil, err
	}
	if host == "" {
		host = database.GetSetting("subHost")
		if host == "" {
			host = "127.0.0.1"
		}
	}
	return BuildShareLinks(&in, c, host)
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
