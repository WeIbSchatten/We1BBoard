package sub

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"github.com/we1bboard/we1bboard/internal/database"
)

type pageData struct {
	Title      string
	SubID      string
	Full       bool // true = show individual share links (?html=1)
	Upload     string
	Download   string
	Total      string
	Used       string
	Remain     string
	Expire     string
	ExpireUnix int64
	Active     bool
	SubURL     string
	ClashURL   string
	SingboxURL string
	JSONURL    string
	QRDataURL  string
	Links      []string
	Emails     []string
	SupportURL string
}

func (s *Server) maybeServeSubPage(c *gin.Context, subID string) bool {
	want, full := wantsSubPage(c)
	if !want {
		return false
	}
	entries, err := s.resolve(subID, requestShareHost(c))
	if err != nil {
		if err == errNotFound {
			c.Status(http.StatusNotFound)
			return true
		}
		c.String(http.StatusInternalServerError, "error")
		return true
	}
	s.serveHTMLPage(c, subID, entries, full)
	return true
}

func (s *Server) serveHTMLPage(c *gin.Context, subID string, entries []subEntry, full bool) {
	urls := ClientSubURLs(subID, c.Request.Host)
	pd, up, down, total := buildPageData(subID, entries, urls, full)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Content-Security-Policy",
		"default-src 'none'; style-src 'unsafe-inline'; img-src 'self' data:; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")

	var buf bytes.Buffer
	if tmpl, err := loadCustomTheme(); err == nil && tmpl != nil {
		if err := tmpl.Execute(&buf, customPageVM(pd, up, down, total)); err != nil {
			// malformed custom theme → fall back to built-in
			buf.Reset()
			_ = subPageTmpl.Execute(&buf, pd)
		}
	} else {
		if err := subPageTmpl.Execute(&buf, pd); err != nil {
			c.String(http.StatusInternalServerError, "error")
			return
		}
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

func buildPageData(subID string, entries []subEntry, urls map[string]string, full bool) (pageData, int64, int64, int64) {
	title := database.GetSetting("subTitle")
	if title == "" {
		title = "We1BBoard"
	}
	support := database.GetSetting("subSupportUrl")
	if support != "" {
		if (!strings.HasPrefix(support, "https://") && !strings.HasPrefix(support, "http://")) ||
			strings.ContainsAny(support, " \t\r\n\"'<>") {
			support = ""
		}
	}

	var up, down, total int64
	var expire int64
	emails := make([]string, 0, len(entries))
	links := make([]string, 0, len(entries))
	for _, e := range entries {
		up += e.Client.Up
		down += e.Client.Down
		if e.Client.TotalGB > 0 {
			t := e.Client.TotalGB * 1024 * 1024 * 1024
			if total == 0 || t < total {
				total = t
			}
		}
		if e.Client.ExpiryTime > 0 {
			sec := e.Client.ExpiryTime / 1000
			if expire == 0 || sec < expire {
				expire = sec
			}
		}
		if e.Name != "" {
			emails = append(emails, e.Name)
		}
		if full && e.Link != "" {
			links = append(links, e.Link)
		}
	}
	used := up + down
	remain := int64(0)
	if total > used {
		remain = total - used
	}

	qrURL := urls["auto"]
	qrData := ""
	if png, err := qrcode.Encode(qrURL, qrcode.Medium, 220); err == nil {
		qrData = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	}

	expireStr := "∞"
	if expire > 0 {
		expireStr = time.Unix(expire, 0).UTC().Format("2006-01-02 15:04 UTC")
	}

	pd := pageData{
		Title:      title,
		SubID:      subID,
		Full:       full,
		Upload:     formatBytes(up),
		Download:   formatBytes(down),
		Total:      formatBytes(total),
		Used:       formatBytes(used),
		Remain:     formatBytes(remain),
		Expire:     expireStr,
		ExpireUnix: expire,
		Active:     len(entries) > 0,
		SubURL:     urls["auto"],
		ClashURL:   urls["clash"],
		SingboxURL: urls["singbox"],
		JSONURL:    urls["json"],
		QRDataURL:  qrData,
		Links:      links,
		Emails:     emails,
		SupportURL: support,
	}
	return pd, up, down, total
}

func formatBytes(n int64) string {
	if n <= 0 {
		return "0 B"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func (s *Server) handleQR(c *gin.Context) {
	subID := c.Param("subId")
	if !ValidSubID(subID) {
		c.Status(http.StatusNotFound)
		return
	}
	// Ensure subId exists (enabled clients) without leaking configs
	entries, err := s.resolve(subID, requestShareHost(c))
	if err != nil || len(entries) == 0 {
		c.Status(http.StatusNotFound)
		return
	}
	url := ClientSubURLs(subID, c.Request.Host)["auto"]
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		c.String(http.StatusInternalServerError, "error")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/png", png)
}

// serveInfoJSON returns live status for custom templates (?format=info), without links.
func (s *Server) serveInfoJSON(c *gin.Context, subID string) {
	entries, err := s.resolve(subID, requestShareHost(c))
	if err != nil {
		if err == errNotFound {
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error"})
		return
	}
	pd, up, down, total := buildPageData(subID, entries, ClientSubURLs(subID, c.Request.Host), false)
	vm := customPageVM(pd, up, down, total)
	delete(vm, "links")
	delete(vm, "Links")
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, vm)
}

var subPageTmpl = template.Must(template.New("subpage").Funcs(template.FuncMap{
	"join": strings.Join,
}).Parse(subPageHTML))

const subPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<meta name="robots" content="noindex,nofollow"/>
<title>{{.Title}} — Subscription</title>
<style>
:root {
  --bg0: #0b1220;
  --bg1: #121a2b;
  --card: #162033;
  --line: #243049;
  --text: #e8eefc;
  --muted: #93a0b8;
  --accent: #3b82f6;
  --accent2: #60a5fa;
  --ok: #34d399;
  --bad: #f87171;
}
* { box-sizing: border-box; }
body {
  margin: 0; min-height: 100vh;
  font-family: "Segoe UI", system-ui, sans-serif;
  color: var(--text);
  background:
    radial-gradient(900px 500px at 10% -10%, rgba(59,130,246,.22), transparent 60%),
    radial-gradient(700px 400px at 100% 0%, rgba(96,165,250,.12), transparent 55%),
    linear-gradient(180deg, var(--bg0), var(--bg1));
}
.wrap { max-width: 720px; margin: 0 auto; padding: 32px 18px 48px; }
.brand {
  font-size: clamp(1.8rem, 4vw, 2.4rem);
  font-weight: 700; letter-spacing: -.02em; margin: 0 0 6px;
}
.sub { color: var(--muted); margin: 0 0 24px; font-size: .95rem; }
.card {
  background: color-mix(in srgb, var(--card) 92%, transparent);
  border: 1px solid var(--line);
  border-radius: 16px;
  padding: 18px 18px 16px;
  margin-bottom: 14px;
  backdrop-filter: blur(8px);
}
.row { display: flex; flex-wrap: wrap; gap: 12px; }
.stat {
  flex: 1 1 140px;
  background: rgba(255,255,255,.03);
  border: 1px solid var(--line);
  border-radius: 12px;
  padding: 12px 14px;
}
.stat .k { color: var(--muted); font-size: .75rem; text-transform: uppercase; letter-spacing: .04em; }
.stat .v { margin-top: 4px; font-size: 1.05rem; font-weight: 600; }
.badge {
  display: inline-block; padding: 4px 10px; border-radius: 999px;
  font-size: .8rem; font-weight: 600;
}
.badge.on { background: rgba(52,211,153,.15); color: var(--ok); }
.badge.off { background: rgba(248,113,113,.15); color: var(--bad); }
.label { color: var(--muted); font-size: .8rem; margin-bottom: 6px; }
.field {
  display: flex; gap: 8px; align-items: stretch; margin-bottom: 10px;
}
.field input {
  flex: 1; min-width: 0;
  background: #0d1524; color: var(--text);
  border: 1px solid var(--line); border-radius: 10px;
  padding: 10px 12px; font-size: .85rem;
}
.btn {
  border: 0; border-radius: 10px; padding: 10px 14px;
  background: var(--accent); color: #fff; font-weight: 600;
  cursor: pointer; white-space: nowrap;
}
.btn:hover { background: var(--accent2); }
.btn.secondary { background: transparent; border: 1px solid var(--line); color: var(--text); }
.qr { display: block; width: 200px; height: 200px; margin: 12px auto 0;
  background: #fff; padding: 10px; border-radius: 12px; }
.links { word-break: break-all; font-size: .8rem; color: var(--muted); }
.links code {
  display: block; background: #0d1524; border: 1px solid var(--line);
  border-radius: 8px; padding: 8px 10px; margin: 6px 0; color: var(--text);
}
.foot { margin-top: 18px; color: var(--muted); font-size: .8rem; text-align: center; }
.foot a { color: var(--accent2); }
.toast {
  position: fixed; bottom: 20px; left: 50%; transform: translateX(-50%);
  background: #1e293b; border: 1px solid var(--line); color: var(--text);
  padding: 8px 14px; border-radius: 999px; font-size: .85rem;
  opacity: 0; pointer-events: none; transition: opacity .2s;
}
.toast.show { opacity: 1; }
</style>
</head>
<body>
<main class="wrap">
  <h1 class="brand">{{.Title}}</h1>
  <p class="sub">Subscription
    {{if .Active}}<span class="badge on">active</span>{{else}}<span class="badge off">inactive</span>{{end}}
  </p>

  <section class="card">
    <div class="row">
      <div class="stat"><div class="k">Used</div><div class="v">{{.Used}}</div></div>
      <div class="stat"><div class="k">Remain</div><div class="v">{{if eq .Total "0 B"}}∞{{else}}{{.Remain}}{{end}}</div></div>
      <div class="stat"><div class="k">Total</div><div class="v">{{if eq .Total "0 B"}}∞{{else}}{{.Total}}{{end}}</div></div>
      <div class="stat"><div class="k">Expire</div><div class="v">{{.Expire}}</div></div>
    </div>
  </section>

  <section class="card">
    <div class="label">Subscription URL</div>
    <div class="field">
      <input id="subUrl" readonly value="{{.SubURL}}"/>
      <button class="btn" type="button" onclick="copy('subUrl')">Copy</button>
    </div>
    <div class="label">Clash</div>
    <div class="field">
      <input id="clashUrl" readonly value="{{.ClashURL}}"/>
      <button class="btn secondary" type="button" onclick="copy('clashUrl')">Copy</button>
    </div>
    <div class="label">sing-box</div>
    <div class="field">
      <input id="sbUrl" readonly value="{{.SingboxURL}}"/>
      <button class="btn secondary" type="button" onclick="copy('sbUrl')">Copy</button>
    </div>
    {{if .QRDataURL}}
    <img class="qr" alt="subscription QR" src="{{.QRDataURL}}"/>
    {{end}}
  </section>

  {{if .Full}}
  <section class="card">
    <div class="label">Configs ({{len .Links}})</div>
    <div class="links">
      {{range .Links}}<code>{{.}}</code>{{end}}
      {{if not .Links}}<p>No active configs.</p>{{end}}
    </div>
  </section>
  {{else}}
  <section class="card">
    <p class="sub" style="margin:0">Open this page in a VPN app via the subscription URL above.
      Config payloads are not embedded here for safety. Append <code>?html=1</code> for the full page.</p>
  </section>
  {{end}}

  <p class="foot">
    Powered by We1BBoard
    {{if .SupportURL}} · <a href="{{.SupportURL}}" rel="noopener noreferrer">Support</a>{{end}}
  </p>
</main>
<div id="toast" class="toast">Copied</div>
<script>
function copy(id) {
  var el = document.getElementById(id);
  if (!el) return;
  var t = el.value;
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(t).then(toast).catch(function(){ fallback(el); });
  } else { fallback(el); }
}
function fallback(el) {
  el.select(); el.setSelectionRange(0, 99999);
  try { document.execCommand('copy'); toast(); } catch (e) {}
}
function toast() {
  var n = document.getElementById('toast');
  n.classList.add('show');
  setTimeout(function(){ n.classList.remove('show'); }, 1200);
}
</script>
</body>
</html>
`
