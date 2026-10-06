package controller

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/panellog"
	"github.com/we1bboard/we1bboard/internal/security"
)

const (
	defaultGeositeURL = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
	defaultGeoipURL   = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
	geodataMaxBytes   = 64 << 20 // 64 MiB per file
)

// GeodataStatus returns sizes and mtimes for geosite.dat / geoip.dat if present.
// GET /server/geodata-status
func (a *API) GeodataStatus(c *gin.Context) {
	ok(c, gin.H{
		"geosite": fileStatus(a.findGeodataFile("geosite.dat")),
		"geoip":   fileStatus(a.findGeodataFile("geoip.dat")),
		"geositeURL": settingOr(database.GetSetting("geodataGeositeURL"), defaultGeositeURL),
		"geoipURL":   settingOr(database.GetSetting("geodataGeoipURL"), defaultGeoipURL),
		"dir":        a.geodataTargetDir(),
	})
}

func fileStatus(path string) gin.H {
	if path == "" {
		return gin.H{"exists": false, "path": ""}
	}
	st, err := os.Stat(path)
	if err != nil {
		return gin.H{"exists": false, "path": path}
	}
	return gin.H{
		"exists": true,
		"path":   path,
		"size":   st.Size(),
		"mtime":  st.ModTime().Unix(),
	}
}

func settingOr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func (a *API) geodataTargetDir() string {
	if a.Xray != nil && a.Xray.ConfigDir != "" {
		return a.Xray.ConfigDir
	}
	if a.Cfg != nil && a.Cfg.BinDir != "" {
		return a.Cfg.BinDir
	}
	if a.Cfg != nil && a.Cfg.DataDir != "" {
		return filepath.Join(a.Cfg.DataDir, "xray")
	}
	return "."
}

// UpdateGeodata downloads geosite.dat and geoip.dat into the xray config/bin dir.
// POST /server/update-geodata
func (a *API) UpdateGeodata(c *gin.Context) {
	geositeURL := settingOr(database.GetSetting("geodataGeositeURL"), defaultGeositeURL)
	geoipURL := settingOr(database.GetSetting("geodataGeoipURL"), defaultGeoipURL)
	if err := security.ValidateNodeURL(geositeURL); err != nil {
		fail(c, 400, fmt.Errorf("geosite URL: %w", err))
		return
	}
	if err := security.ValidateNodeURL(geoipURL); err != nil {
		fail(c, 400, fmt.Errorf("geoip URL: %w", err))
		return
	}
	dir := a.geodataTargetDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(c, 500, err)
		return
	}
	client := security.SafeDownloadClient(false, 120*time.Second)
	gsPath := filepath.Join(dir, "geosite.dat")
	giPath := filepath.Join(dir, "geoip.dat")
	gsBytes, err := downloadToFile(client, geositeURL, gsPath)
	if err != nil {
		fail(c, 502, fmt.Errorf("geosite: %w", err))
		return
	}
	giBytes, err := downloadToFile(client, geoipURL, giPath)
	if err != nil {
		fail(c, 502, fmt.Errorf("geoip: %w", err))
		return
	}
	panellog.Append("geodata updated: geosite=%d geoip=%d dir=%s", gsBytes, giBytes, dir)
	ok(c, gin.H{
		"geosite": fileStatus(gsPath),
		"geoip":   fileStatus(giPath),
		"message": "geodata updated; restart xray if already running",
	})
}

func downloadToFile(client *http.Client, rawURL, dest string) (int64, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, geodataMaxBytes+1))
	_ = f.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	if n > geodataMaxBytes {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("file exceeds %d bytes", geodataMaxBytes)
	}
	if n < 100 {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("file too small")
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	return n, nil
}

// WarpGenerate returns a WireGuard keypair + draft Cloudflare WARP-like outbound settings.
// POST /xray/warp/generate
func (a *API) WarpGenerate(c *gin.Context) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		fail(c, 500, err)
		return
	}
	pub := priv.PublicKey()
	privateKey := base64.StdEncoding.EncodeToString(priv.Bytes())
	publicKey := base64.StdEncoding.EncodeToString(pub.Bytes())

	settings := map[string]any{
		"secretKey": privateKey,
		"address":   []string{}, // fill from warp-cli / registration (e.g. 172.16.0.2/32)
		"peers": []map[string]any{{
			"publicKey":  "", // Cloudflare WARP peer public key from registration
			"endpoint":   "engage.cloudflareclient.com:2408",
			"allowedIPs": []string{"0.0.0.0/0", "::/0"},
		}},
		"reserved": []int{}, // 3-byte reserved from WARP registration
		"mtu":      1280,
	}
	settingsJSON, _ := json.MarshalIndent(settings, "", "  ")
	ok(c, gin.H{
		"privateKey": privateKey,
		"publicKey":  publicKey,
		"settings":   json.RawMessage(settingsJSON),
		"note":       "Fill address, peers[0].publicKey, and reserved from Cloudflare WARP registration (warp-cli or wgcf). Keys alone are not enough.",
		"outbound": gin.H{
			"tag":            "warp",
			"protocol":       "wireguard",
			"enable":         true,
			"remark":         "Cloudflare WARP (placeholder)",
			"settings":       string(settingsJSON),
			"streamSettings": "",
		},
	})
}

// WarpApply generates a WARP WireGuard keypair and creates a disabled outbound (tag warp).
// User only needs to fill address / peer publicKey / reserved from Cloudflare.
// POST /xray/warp/apply
func (a *API) WarpApply(c *gin.Context) {
	if a.Outbound == nil {
		fail(c, 500, fmt.Errorf("outbound service unavailable"))
		return
	}
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		fail(c, 500, err)
		return
	}
	privateKey := base64.StdEncoding.EncodeToString(priv.Bytes())
	publicKey := base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes())

	settings := map[string]any{
		"secretKey": privateKey,
		"address":   []string{},
		"peers": []map[string]any{{
			"publicKey":  "",
			"endpoint":   "engage.cloudflareclient.com:2408",
			"allowedIPs": []string{"0.0.0.0/0", "::/0"},
		}},
		"reserved": []int{},
		"mtu":      1280,
	}
	settingsJSON, _ := json.MarshalIndent(settings, "", "  ")

	tag := "warp"
	var existing []model.Outbound
	_ = database.DB.Select("tag").Find(&existing).Error
	used := map[string]bool{}
	for _, o := range existing {
		used[o.Tag] = true
	}
	if used[tag] {
		for i := 2; ; i++ {
			cand := fmt.Sprintf("warp-%d", i)
			if !used[cand] {
				tag = cand
				break
			}
		}
	}

	o := &model.Outbound{
		Tag:            tag,
		Protocol:       "wireguard",
		Settings:       string(settingsJSON),
		StreamSettings: "",
		Enable:         false,
		Remark:         "Cloudflare WARP (fill address/reserved)",
	}
	if err := a.Outbound.Create(o); err != nil {
		fail(c, 400, err)
		return
	}
	panellog.Append("warp apply: created outbound tag=%s (disabled)", tag)
	ok(c, gin.H{
		"outbound":   o,
		"privateKey": privateKey,
		"publicKey":  publicKey,
		"note":       "Outbound created disabled. Fill address, peers[0].publicKey, and reserved from Cloudflare WARP registration, then enable.",
	})
}
