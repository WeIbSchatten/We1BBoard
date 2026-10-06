package job

import (
	"fmt"
	"log"
	"time"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/discordnotify"
	"github.com/we1bboard/we1bboard/internal/emailnotify"
	"github.com/we1bboard/we1bboard/internal/panellog"
	"github.com/we1bboard/we1bboard/internal/tgnotify"
)

// StartTrafficReset runs hourly: period traffic resets + 80% totalGB warnings.
func StartTrafficReset() {
	go func() {
		// Slight delay so DB/xray settle after boot.
		time.Sleep(15 * time.Second)
		runTrafficJobs()
		t := time.NewTicker(1 * time.Hour)
		defer t.Stop()
		for range t.C {
			runTrafficJobs()
		}
	}()
	log.Println("[job] traffic reset started")
}

func runTrafficJobs() {
	if database.DB == nil {
		return
	}
	resetDueTraffic()
	warnTrafficThreshold()
}

func resetDueTraffic() {
	var clients []model.Client
	if err := database.DB.Where("traffic_reset IN ?", []string{"daily", "weekly", "monthly"}).Find(&clients).Error; err != nil {
		return
	}
	now := time.Now()
	nowMs := now.UnixMilli()
	for i := range clients {
		c := &clients[i]
		if !shouldResetTraffic(c, now) {
			continue
		}
		usedBefore := c.Up + c.Down
		res := database.DB.Model(&model.Client{}).Where("id = ?", c.ID).Updates(map[string]any{
			"up":                 0,
			"down":               0,
			"last_traffic_reset": nowMs,
		})
		if res.Error != nil || res.RowsAffected == 0 {
			continue
		}
		c.Up, c.Down = 0, 0
		c.LastTrafficReset = nowMs
		ClearTraffic80Notified(c.Email)
		panellog.Append("trafficReset: email=%s period=%s used_before=%d", c.Email, c.TrafficReset, usedBefore)
		notifyTraffic(fmt.Sprintf(
			"We1BBoard traffic reset\nemail: %s\nperiod: %s\nused before: %.2f GB\ntime: %s",
			c.Email, c.TrafficReset, float64(usedBefore)/(1024*1024*1024), now.UTC().Format(time.RFC3339),
		))
	}
}

func shouldResetTraffic(c *model.Client, now time.Time) bool {
	last := time.UnixMilli(c.LastTrafficReset)
	switch model.NormalizeTrafficReset(c.TrafficReset) {
	case "daily":
		if c.LastTrafficReset <= 0 {
			return true
		}
		ly, lm, ld := last.Date()
		ny, nm, nd := now.Date()
		return ly != ny || lm != nm || ld != nd
	case "weekly":
		if c.LastTrafficReset <= 0 {
			return true
		}
		return now.Sub(last) > 7*24*time.Hour
	case "monthly":
		if c.LastTrafficReset <= 0 {
			return true
		}
		if now.Sub(last) > 30*24*time.Hour {
			return true
		}
		ly, lm, _ := last.Date()
		ny, nm, _ := now.Date()
		return ly != ny || lm != nm
	default:
		return false
	}
}

func warnTrafficThreshold() {
	var clients []model.Client
	if err := database.DB.Where("total_gb > 0").Find(&clients).Error; err != nil {
		return
	}
	for _, c := range clients {
		limitBytes := c.TotalGB * 1e9
		if limitBytes <= 0 {
			continue
		}
		used := c.Up + c.Down
		if used < int64(float64(limitBytes)*0.8) {
			ClearTraffic80Notified(c.Email)
			continue
		}
		if used >= limitBytes {
			continue // depleted — separate status; avoid double spam with 80%
		}
		if WasTraffic80Notified(c.Email) {
			continue
		}
		MarkTraffic80Notified(c.Email)
		pct := float64(used) / float64(limitBytes) * 100
		panellog.Append("trafficWarn80: email=%s used=%.1f%% totalGB=%d", c.Email, pct, c.TotalGB)
		notifyTraffic(fmt.Sprintf(
			"We1BBoard traffic warning (80%%)\nemail: %s\nused: %.2f / %d GB (%.0f%%)\ntime: %s",
			c.Email,
			float64(used)/(1024*1024*1024),
			c.TotalGB,
			pct,
			time.Now().UTC().Format(time.RFC3339),
		))
	}
}

func notifyTraffic(msg string) {
	go tgnotify.NotifyTraffic(msg)
	go emailnotify.NotifyTraffic(msg)
	go discordnotify.NotifyTraffic(msg)
}
