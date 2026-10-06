// Package rates tracks inbound traffic speed from successive up/down samples.
package rates

import (
	"sync"
	"time"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
)

// Sample is one inbound's instantaneous rates in bytes/sec.
type Sample struct {
	ID       uint    `json:"id"`
	UpRate   float64 `json:"upRate"`
	DownRate float64 `json:"downRate"`
}

type prev struct {
	up   int64
	down int64
	at   time.Time
}

var (
	mu      sync.RWMutex
	last    = map[uint]prev{}
	current = map[uint]Sample{}
)

// SampleFromDB reads inbound up/down counters and updates in-memory rates.
func SampleFromDB() {
	if database.DB == nil {
		return
	}
	var rows []model.Inbound
	if err := database.DB.Select("id", "up", "down").Find(&rows).Error; err != nil {
		return
	}
	now := time.Now()
	mu.Lock()
	defer mu.Unlock()
	seen := make(map[uint]struct{}, len(rows))
	for _, r := range rows {
		seen[r.ID] = struct{}{}
		p, ok := last[r.ID]
		s := Sample{ID: r.ID}
		if ok && now.After(p.at) {
			dt := now.Sub(p.at).Seconds()
			if dt > 0.2 {
				if r.Up >= p.up {
					s.UpRate = float64(r.Up-p.up) / dt
				}
				if r.Down >= p.down {
					s.DownRate = float64(r.Down-p.down) / dt
				}
			}
		}
		last[r.ID] = prev{up: r.Up, down: r.Down, at: now}
		current[r.ID] = s
	}
	for id := range last {
		if _, ok := seen[id]; !ok {
			delete(last, id)
			delete(current, id)
		}
	}
}

// All returns a copy of the latest rate samples.
func All() []Sample {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Sample, 0, len(current))
	for _, s := range current {
		out = append(out, s)
	}
	return out
}
