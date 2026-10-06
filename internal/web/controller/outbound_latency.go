package controller

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/web/observatory"
)

// OutboundTestResult is the TCP dial latency for one outbound.
type OutboundTestResult struct {
	ID        uint    `json:"id"`
	Tag       string  `json:"tag"`
	OK        bool    `json:"ok"`
	LatencyMs float64 `json:"latencyMs"`
	Error     string  `json:"error,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// TestOutbound dials the proxy address:port from outbound settings (5s timeout).
// POST /outbounds/:id/test
func (a *API) TestOutbound(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var o model.Outbound
	if err := database.DB.First(&o, id).Error; err != nil {
		fail(c, 404, fmt.Errorf("outbound not found"))
		return
	}
	ok(c, dialOutbound(&o))
}

// TestAllOutbounds tests all proxy outbounds.
// POST /outbounds/test-all
func (a *API) TestAllOutbounds(c *gin.Context) {
	var rows []model.Outbound
	if err := database.DB.Order("id asc").Find(&rows).Error; err != nil {
		fail(c, 500, err)
		return
	}
	results := make([]OutboundTestResult, 0, len(rows))
	obsItems := make([]observatory.Item, 0, len(rows))
	for i := range rows {
		o := &rows[i]
		if !isProxyOutbound(o.Protocol, o.Tag) {
			continue
		}
		r := dialOutbound(o)
		results = append(results, r)
		obsItems = append(obsItems, observatory.Item{Tag: r.Tag, Delay: r.LatencyMs, Alive: r.OK})
	}
	observatory.Store(obsItems)
	ok(c, gin.H{"results": results})
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

func dialOutbound(o *model.Outbound) OutboundTestResult {
	res := OutboundTestResult{ID: o.ID, Tag: o.Tag}
	addr, port, err := extractOutboundAddr(o.Protocol, o.Settings)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	target := net.JoinHostPort(addr, strconv.Itoa(port))
	res.Address = target
	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		res.Error = err.Error()
		res.LatencyMs = float64(time.Since(start).Milliseconds())
		return res
	}
	_ = conn.Close()
	res.OK = true
	res.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
	return res
}

func extractOutboundAddr(protocol, settingsJSON string) (string, int, error) {
	var s map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &s); err != nil {
		return "", 0, fmt.Errorf("invalid settings")
	}
	proto := strings.ToLower(protocol)

	// WireGuard: peers[0].endpoint "host:port"
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
		return "", 0, fmt.Errorf("no wireguard endpoint")
	}

	// vless/vmess: vnext[]; trojan/ss/socks/http: servers[]
	listKey := "servers"
	if proto == "vless" || proto == "vmess" {
		listKey = "vnext"
	}
	list, _ := s[listKey].([]any)
	if len(list) == 0 {
		// some forms put address/port at top level
		if addr, _ := s["address"].(string); addr != "" {
			port := intFromAny(s["port"], 0)
			if port > 0 {
				return addr, port, nil
			}
		}
		return "", 0, fmt.Errorf("no server address")
	}
	first, _ := list[0].(map[string]any)
	if first == nil {
		return "", 0, fmt.Errorf("no server address")
	}
	addr, _ := first["address"].(string)
	port := intFromAny(first["port"], 0)
	if addr == "" || port <= 0 {
		return "", 0, fmt.Errorf("no server address")
	}
	return addr, port, nil
}

func intFromAny(v any, def int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return def
		}
		return int(i)
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
