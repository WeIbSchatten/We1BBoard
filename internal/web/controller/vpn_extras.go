package controller

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/we1bboard/we1bboard/internal/web/observatory"
	"github.com/we1bboard/we1bboard/internal/xray"
)

// XrayObservatory returns the last observatory-lite probe snapshot.
// GET /xray/observatory
func (a *API) XrayObservatory(c *gin.Context) {
	ok(c, observatory.Get())
}

// NordTemplate returns a draft WireGuard outbound placeholder for NordVPN.
// POST /xray/nord/template
func (a *API) NordTemplate(c *gin.Context) {
	settings := map[string]any{
		"secretKey": "",
		"address":   []string{},
		"peers": []map[string]any{{
			"publicKey":  "",
			"endpoint":   "xx.nordvpn.com:51820",
			"allowedIPs": []string{"0.0.0.0/0", "::/0"},
		}},
		"mtu": 1420,
	}
	settingsJSON, _ := json.MarshalIndent(settings, "", "  ")
	ok(c, gin.H{
		"note": "NordVPN recommends WireGuard. Fill secretKey, address, and peers[0] from your Nord account / config download. OpenVPN is not natively supported as an Xray outbound.",
		"outbound": gin.H{
			"tag":            "nordvpn",
			"protocol":       "wireguard",
			"enable":         false,
			"remark":         "NordVPN WireGuard placeholder — fill keys from Nord config",
			"settings":       string(settingsJSON),
			"streamSettings": "",
		},
	})
}

// PIATemplate returns a draft WireGuard outbound placeholder for Private Internet Access.
// POST /xray/pia/template
func (a *API) PIATemplate(c *gin.Context) {
	settings := map[string]any{
		"secretKey": "",
		"address":   []string{},
		"peers": []map[string]any{{
			"publicKey":  "",
			"endpoint":   "xx.privacy.network:1337",
			"allowedIPs": []string{"0.0.0.0/0", "::/0"},
		}},
		"mtu": 1420,
	}
	settingsJSON, _ := json.MarshalIndent(settings, "", "  ")
	ok(c, gin.H{
		"note": "PIA WireGuard placeholder. Fill secretKey, address, and peers[0] from PIA account / WireGuard config generator.",
		"outbound": gin.H{
			"tag":            "pia",
			"protocol":       "wireguard",
			"enable":         false,
			"remark":         "PIA WireGuard placeholder — fill keys from PIA config",
			"settings":       string(settingsJSON),
			"streamSettings": "",
		},
	})
}

// AmneziaWGLogs is a stub: AmneziaWG runs inside xray; use process/error logs.
// GET /logs/amneziawg
func (a *API) AmneziaWGLogs(c *gin.Context) {
	note := "AmneziaWG runs inside xray; see process/error logs"
	lines := []string{}
	if a.Xray != nil {
		path := a.Xray.ProcessLogPath()
		if path != "" {
			if out, err := xray.TailFile(path, 50); err == nil {
				for _, line := range out {
					low := strings.ToLower(line)
					if strings.Contains(low, "amnezia") {
						lines = append(lines, line)
					}
				}
			}
		}
	}
	ok(c, gin.H{"lines": lines, "note": note})
}
