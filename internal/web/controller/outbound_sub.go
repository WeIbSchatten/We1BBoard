package controller

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/link"
	"github.com/we1bboard/we1bboard/internal/panellog"
	"github.com/we1bboard/we1bboard/internal/security"
)

// ListOutboundSubs GET /outbound-subs
func (a *API) ListOutboundSubs(c *gin.Context) {
	var rows []model.OutboundSubscription
	if err := database.DB.Order("id asc").Find(&rows).Error; err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, rows)
}

// CreateOutboundSub POST /outbound-subs
func (a *API) CreateOutboundSub(c *gin.Context) {
	var s model.OutboundSubscription
	if err := c.ShouldBindJSON(&s); err != nil {
		fail(c, 400, err)
		return
	}
	s.ID = 0
	if err := validateOutboundSub(&s); err != nil {
		fail(c, 400, err)
		return
	}
	if s.Prefix == "" {
		s.Prefix = "sub-"
	}
	if err := database.DB.Create(&s).Error; err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, s)
}

// UpdateOutboundSub PUT /outbound-subs/:id
func (a *API) UpdateOutboundSub(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var s model.OutboundSubscription
	if err := c.ShouldBindJSON(&s); err != nil {
		fail(c, 400, err)
		return
	}
	s.ID = uint(id)
	if err := validateOutboundSub(&s); err != nil {
		fail(c, 400, err)
		return
	}
	if s.Prefix == "" {
		s.Prefix = "sub-"
	}
	if err := database.DB.Save(&s).Error; err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, s)
}

// DeleteOutboundSub DELETE /outbound-subs/:id
func (a *API) DeleteOutboundSub(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := database.DB.Delete(&model.OutboundSubscription{}, id).Error; err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}

// RefreshOutboundSub POST /outbound-subs/:id/refresh
func (a *API) RefreshOutboundSub(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var s model.OutboundSubscription
	if err := database.DB.First(&s, id).Error; err != nil {
		fail(c, 404, fmt.Errorf("subscription not found"))
		return
	}
	n, err := a.refreshOutboundSub(&s)
	if err != nil {
		s.LastError = err.Error()
		s.LastFetch = time.Now().Unix()
		_ = database.DB.Save(&s).Error
		fail(c, 502, err)
		return
	}
	ok(c, gin.H{"count": n, "sub": s})
}

func validateOutboundSub(s *model.OutboundSubscription) error {
	s.URL = strings.TrimSpace(s.URL)
	s.Remark = strings.TrimSpace(s.Remark)
	s.Prefix = strings.TrimSpace(s.Prefix)
	if s.URL == "" {
		return fmt.Errorf("url required")
	}
	if err := security.ValidateNodeURL(s.URL); err != nil {
		return err
	}
	if s.IntervalMin < 0 {
		return fmt.Errorf("intervalMin must be >= 0")
	}
	if len(s.Prefix) > 64 {
		return fmt.Errorf("prefix too long")
	}
	return nil
}

func (a *API) refreshOutboundSub(s *model.OutboundSubscription) (int, error) {
	client := security.SafeHTTPClient(false, 60*time.Second)
	resp, err := client.Get(s.URL)
	if err != nil {
		return 0, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, fetchSubMaxBytes+1))
	if err != nil {
		return 0, fmt.Errorf("read: %w", err)
	}
	if len(body) > fetchSubMaxBytes {
		return 0, fmt.Errorf("body exceeds %d bytes", fetchSubMaxBytes)
	}
	links := link.ExtractShareLinks(string(body))
	if len(links) == 0 {
		s.LastError = "no share links found"
		s.LastFetch = time.Now().Unix()
		_ = database.DB.Save(s).Error
		return 0, fmt.Errorf("no share links found")
	}
	prefix := s.Prefix
	if prefix == "" {
		prefix = "sub-"
	}

	// Remove previous outbounds for this prefix (except system tags).
	var existing []model.Outbound
	_ = database.DB.Find(&existing).Error
	for _, o := range existing {
		if o.Tag == "direct" || o.Tag == "blocked" || o.Tag == "api" {
			continue
		}
		if strings.HasPrefix(o.Tag, prefix) {
			_ = database.DB.Delete(&model.Outbound{}, o.ID).Error
		}
	}

	created := 0
	var lastErr string
	for i, raw := range links {
		parsed, err := link.ParseOutboundLink(raw)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		tag := fmt.Sprintf("%s%d", prefix, i+1)
		remark := parsed.Remark
		if remark == "" {
			remark = parsed.Tag
		}
		if remark == "" {
			remark = tag
		}
		o := model.Outbound{
			Tag:            tag,
			Protocol:       parsed.Protocol,
			Settings:       parsed.Settings,
			StreamSettings: parsed.StreamSettings,
			Enable:         s.Enable,
			Remark:         remark,
		}
		if err := database.DB.Create(&o).Error; err != nil {
			lastErr = err.Error()
			continue
		}
		created++
	}
	s.LastFetch = time.Now().Unix()
	if created == 0 {
		s.LastError = lastErr
		if s.LastError == "" {
			s.LastError = "parse failed for all links"
		}
		_ = database.DB.Save(s).Error
		return 0, fmt.Errorf("%s", s.LastError)
	}
	s.LastError = ""
	_ = database.DB.Save(s).Error
	panellog.Append("outbound-sub %d refreshed: %d outbounds", s.ID, created)
	if a.RT != nil {
		_ = a.RT.ReloadLocal()
	}
	return created, nil
}

// RefreshDueOutboundSubs is called by the background job for IntervalMin > 0.
func (a *API) RefreshDueOutboundSubs() {
	var rows []model.OutboundSubscription
	if err := database.DB.Where("enable = ? AND interval_min > 0", true).Find(&rows).Error; err != nil {
		return
	}
	now := time.Now().Unix()
	for i := range rows {
		s := &rows[i]
		intervalSec := int64(s.IntervalMin) * 60
		if s.LastFetch > 0 && now-s.LastFetch < intervalSec {
			continue
		}
		if _, err := a.refreshOutboundSub(s); err != nil {
			panellog.Append("outbound-sub %d auto-refresh: %v", s.ID, err)
		}
	}
}
