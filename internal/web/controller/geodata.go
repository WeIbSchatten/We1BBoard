package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// Common geosite/geoip category tags (3x-ui / Loyalsoldier style).
var staticGeosite = []string{
	"google", "netflix", "telegram", "facebook", "twitter", "youtube",
	"category-ads-all", "category-ads", "cn", "geolocation-cn", "geolocation-!cn",
	"cloudflare", "cloudfront", "amazon", "apple", "microsoft", "github",
	"openai", "discord", "tiktok", "instagram", "whatsapp", "spotify",
	"steam", "twitch", "bahamut", "bilibili", "gfw", "greatfire",
	"category-porn", "tld-cn", "private",
}

var staticGeoip = []string{
	"cn", "private", "cloudflare", "cloudfront", "facebook", "google",
	"netflix", "telegram", "twitter", "amazon", "microsoft", "fastly",
	"tor", "ir", "ru", "us", "jp", "kr", "hk", "tw", "sg", "de", "gb",
}

// GeodataList returns category names for geosite/geoip.
// GET /geodata/list?type=geosite|geoip
// Reads .dat if present next to xray bin or in ConfigDir; otherwise returns a curated static list.
func (a *API) GeodataList(c *gin.Context) {
	typ := strings.ToLower(strings.TrimSpace(c.DefaultQuery("type", "geosite")))
	if typ != "geosite" && typ != "geoip" {
		fail(c, 400, fmt.Errorf("type must be geosite or geoip"))
		return
	}
	datName := typ + ".dat"
	datPath := a.findGeodataFile(datName)
	tags := staticGeosite
	if typ == "geoip" {
		tags = staticGeoip
	}
	// Full .dat parsing needs v2fly libraries; return curated static tags and note file presence.
	ok(c, gin.H{
		"type":   typ,
		"tags":   tags,
		"source": "static",
		"path":   datPath,
		"note":   "category names from curated static list; place geosite.dat/geoip.dat next to xray for runtime matching",
	})
}

func (a *API) findGeodataFile(name string) string {
	candidates := []string{}
	if a.Xray != nil {
		if a.Xray.Bin != "" {
			candidates = append(candidates, filepath.Join(filepath.Dir(a.Xray.Bin), name))
		}
		if a.Xray.ConfigDir != "" {
			candidates = append(candidates, filepath.Join(a.Xray.ConfigDir, name))
		}
	}
	if a.Cfg != nil {
		if a.Cfg.BinDir != "" {
			candidates = append(candidates, filepath.Join(a.Cfg.BinDir, name))
		}
		if a.Cfg.DataDir != "" {
			candidates = append(candidates, filepath.Join(a.Cfg.DataDir, name))
			candidates = append(candidates, filepath.Join(a.Cfg.DataDir, "xray", name))
		}
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}
