// Package observatory keeps a lite in-memory snapshot of outbound probe results.
package observatory

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/xray"
)

// Item is one outbound probe sample.
type Item struct {
	Tag   string  `json:"tag"`
	Delay float64 `json:"delay"`
	Alive bool    `json:"alive"`
}

// Snapshot is returned by GET /xray/observatory.
type Snapshot struct {
	UpdatedAt int64  `json:"updatedAt"`
	Items     []Item `json:"items"`
}

var (
	mu   sync.RWMutex
	last Snapshot
	once sync.Once
)

// Start launches the 30s sampler when observatory is present in the template (idempotent).
func Start() {
	once.Do(func() {
		go loop()
	})
}

func loop() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	sample()
	for range t.C {
		sample()
	}
}

func sample() {
	tpl := xray.LoadTemplate()
	if tpl["observatory"] == nil && tpl["burstObservatory"] == nil {
		return
	}
	results := probeAll()
	mu.Lock()
	last = Snapshot{UpdatedAt: time.Now().Unix(), Items: results}
	mu.Unlock()
}

// Store replaces the snapshot (e.g. after manual test-all).
func Store(items []Item) {
	mu.Lock()
	defer mu.Unlock()
	last = Snapshot{UpdatedAt: time.Now().Unix(), Items: append([]Item(nil), items...)}
}

// Get returns a copy of the last snapshot.
func Get() Snapshot {
	mu.RLock()
	defer mu.RUnlock()
	return Snapshot{
		UpdatedAt: last.UpdatedAt,
		Items:     append([]Item(nil), last.Items...),
	}
}

func probeAll() []Item {
	var rows []model.Outbound
	_ = database.DB.Order("id asc").Find(&rows)
	out := make([]Item, 0, len(rows))
	for i := range rows {
		o := &rows[i]
		if !isProxyOutbound(o.Protocol, o.Tag) {
			continue
		}
		item := Item{Tag: o.Tag}
		addr, port, err := extractAddr(o.Protocol, o.Settings)
		if err != nil {
			out = append(out, item)
			continue
		}
		target := net.JoinHostPort(addr, strconv.Itoa(port))
		start := time.Now()
		conn, err := net.DialTimeout("tcp", target, 5*time.Second)
		if err != nil {
			item.Delay = float64(time.Since(start).Milliseconds())
			out = append(out, item)
			continue
		}
		_ = conn.Close()
		item.Alive = true
		item.Delay = float64(time.Since(start).Microseconds()) / 1000.0
		out = append(out, item)
	}
	return out
}

func isProxyOutbound(protocol, tag string) bool {
	switch strings.ToLower(protocol) {
	case "freedom", "blackhole", "dns", "block":
		return false
	}
	switch tag {
	case "direct", "blocked", "block", "blackhole":
		return false
	}
	return true
}

func extractAddr(protocol, settingsJSON string) (string, int, error) {
	var s map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &s); err != nil {
		return "", 0, err
	}
	proto := strings.ToLower(protocol)
	if proto == "wireguard" {
		peers, _ := s["peers"].([]any)
		if len(peers) > 0 {
			if p, ok := peers[0].(map[string]any); ok {
				ep, _ := p["endpoint"].(string)
				if host, portStr, err := net.SplitHostPort(ep); err == nil {
					port, _ := strconv.Atoi(portStr)
					if host != "" && port > 0 {
						return host, port, nil
					}
				}
			}
		}
		return "", 0, errNoAddr
	}
	listKey := "servers"
	if proto == "vless" || proto == "vmess" {
		listKey = "vnext"
	}
	list, _ := s[listKey].([]any)
	if len(list) == 0 {
		if addr, _ := s["address"].(string); addr != "" {
			port := intFromAny(s["port"], 0)
			if port > 0 {
				return addr, port, nil
			}
		}
		return "", 0, errNoAddr
	}
	first, _ := list[0].(map[string]any)
	if first == nil {
		return "", 0, errNoAddr
	}
	addr, _ := first["address"].(string)
	port := intFromAny(first["port"], 0)
	if addr == "" || port <= 0 {
		return "", 0, errNoAddr
	}
	return addr, port, nil
}

var errNoAddr = &simpleErr{"no server address"}

type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }

func intFromAny(v any, def int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return int(n)
	case int64:
		return int(n)
	case string:
		i, err := strconv.Atoi(n)
		if err != nil {
			return def
		}
		return i
	default:
		return def
	}
}
