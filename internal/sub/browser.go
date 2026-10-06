package sub

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// vpnClientUA markers: clients that send Mozilla/ but must get raw subscription body.
var vpnClientUA = []string{
	"clash", "mihomo", "stash", "surge", "quantumult", "loon", "nuko",
	"sing-box", "singbox", "sfa", "sfi", "sfm", "sft",
	"v2ray", "xray", "v2box", "nekobox", "hiddify", "shadowrocket",
	"streisand", "passwall", "openclash", "fairvpn", "v2rayng", "v2rayn",
	"kitsunebi", "pharos", "napsternet", "oneclick", "surfboard",
}

// explicitHTMLRequest is an operator asking for the full themed page (?html=1).
func explicitHTMLRequest(c *gin.Context) bool {
	if c.Query("html") == "1" {
		return true
	}
	return strings.EqualFold(c.Query("view"), "html")
}

// isBrowserSubscriptionRequest detects document navigations from browsers.
// VPN clients that spoof Mozilla/ stay on the raw body via vpnClientUA allowlist.
func isBrowserSubscriptionRequest(c *gin.Context) bool {
	if explicitHTMLRequest(c) {
		return true
	}
	dest := strings.ToLower(c.GetHeader("Sec-Fetch-Dest"))
	mode := strings.ToLower(c.GetHeader("Sec-Fetch-Mode"))
	if dest == "document" || mode == "navigate" {
		return true
	}
	accept := strings.ToLower(c.GetHeader("Accept"))
	if strings.Contains(accept, "text/html") {
		return true
	}
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	if !strings.Contains(ua, "mozilla/") {
		return false
	}
	for _, m := range vpnClientUA {
		if strings.Contains(ua, m) {
			return false
		}
	}
	return true
}

// wantsSubPage reports whether to render HTML and whether full (links) page is allowed.
// Implicit browser nav → copy-only (no embedded proxy configs). Explicit ?html=1 → full.
func wantsSubPage(c *gin.Context) (want bool, full bool) {
	if explicitHTMLRequest(c) {
		return true, true
	}
	if isBrowserSubscriptionRequest(c) {
		return true, false
	}
	return false, false
}
