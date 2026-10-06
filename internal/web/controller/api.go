package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	psnet "github.com/shirou/gopsutil/v4/net"
	"github.com/skip2/go-qrcode"
	"github.com/we1bboard/we1bboard/internal/bridge"
	"github.com/we1bboard/we1bboard/internal/clientonline"
	"github.com/we1bboard/we1bboard/internal/config"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/panellog"
	"github.com/we1bboard/we1bboard/internal/protocol"
	"github.com/we1bboard/we1bboard/internal/security"
	"github.com/we1bboard/we1bboard/internal/sub"
	"github.com/we1bboard/we1bboard/internal/discordnotify"
	"github.com/we1bboard/we1bboard/internal/emailnotify"
	"github.com/we1bboard/we1bboard/internal/tgnotify"
	"github.com/we1bboard/we1bboard/internal/tgproxy"
	"github.com/we1bboard/we1bboard/internal/web/middleware"
	"github.com/we1bboard/we1bboard/internal/web/nodehist"
	"github.com/we1bboard/we1bboard/internal/web/rates"
	"github.com/we1bboard/we1bboard/internal/web/runtime"
	"github.com/we1bboard/we1bboard/internal/web/service"
	"github.com/we1bboard/we1bboard/internal/xray"
)

type API struct {
	Auth     *service.AuthService
	Inbound  *service.InboundService
	Client   *service.ClientService
	Host     *service.HostService
	Outbound *service.OutboundService
	Node     *service.NodeService
	Routing  *service.RoutingService
	Bridge   *bridge.Service
	TgProxy  *tgproxy.Manager
	Xray     *xray.Manager
	RT       *runtime.Hub
	Cfg      *config.Config
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func fail(c *gin.Context, code int, err error) {
	c.JSON(code, gin.H{"success": false, "error": err.Error()})
}

func (a *API) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,max=64"`
		Password string `json:"password" binding:"required,max=128"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	if !middleware.LoginUserLimit(req.Username) {
		fail(c, 429, fmt.Errorf("too many login attempts, try later"))
		return
	}
	u, err := a.Auth.Login(req.Username, req.Password)
	if err != nil {
		// uniform message — no user enumeration
		fail(c, 401, fmt.Errorf("invalid credentials"))
		return
	}
	if settingTruthy(database.GetSetting("twoFactorEnable")) {
		secret := database.GetSetting("twoFactorSecret")
		code := strings.TrimSpace(req.Code)
		if code == "" {
			fail(c, 401, fmt.Errorf("two-factor code required"))
			return
		}
		if secret == "" || !totp.Validate(code, secret) {
			fail(c, 401, fmt.Errorf("invalid two-factor code"))
			return
		}
	}
	sess := sessions.Default(c)
	sess.Clear()
	sess.Options(sessions.Options{
		Path:     "/",
		HttpOnly: true,
		MaxAge:   7 * 24 * 3600,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request.TLS != nil || (config.Env("WE1B_TRUST_PROXY", "") == "1" && c.GetHeader("X-Forwarded-Proto") == "https"),
	})
	sess.Set("uid", u.ID)
	sess.Set("username", u.Username)
	_ = sess.Save()
	ip := c.ClientIP()
	go tgnotify.NotifyLogin(u.Username, ip)
	go emailnotify.NotifyLogin(u.Username, ip)
	go discordnotify.NotifyLogin(u.Username, ip)
	ok(c, gin.H{"username": u.Username})
}

func (a *API) Logout(c *gin.Context) {
	sess := sessions.Default(c)
	sess.Clear()
	_ = sess.Save()
	ok(c, nil)
}

func (a *API) Me(c *gin.Context) {
	sess := sessions.Default(c)
	ok(c, gin.H{"username": sess.Get("username")})
}

func (a *API) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if len(auth) > 7 && auth[:7] == "Bearer " {
			token := auth[7:]
			expected := database.GetSetting("nodeToken")
			if security.EqualSecret(token, expected) {
				c.Set("authMode", "node")
				c.Next()
				return
			}
			c.AbortWithStatusJSON(401, gin.H{"success": false, "error": "unauthorized"})
			return
		}
		sess := sessions.Default(c)
		if sess.Get("uid") == nil {
			c.AbortWithStatusJSON(401, gin.H{"success": false, "error": "unauthorized"})
			return
		}
		c.Set("authMode", "session")
		c.Next()
	}
}

// RequireSession blocks node-token callers from admin-only endpoints (password, settings, …).
func (a *API) RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("authMode") != "session" {
			c.AbortWithStatusJSON(403, gin.H{"success": false, "error": "session required"})
			return
		}
		c.Next()
	}
}

func (a *API) ListProtocols(c *gin.Context) {
	type info struct {
		Protocol string `json:"protocol"`
		Engine   string `json:"engine"`
	}
	out := []info{}
	for _, p := range protocol.All() {
		adap, _ := protocol.Get(p)
		out = append(out, info{Protocol: string(p), Engine: adap.Engine()})
	}
	ok(c, out)
}

func (a *API) ListInbounds(c *gin.Context) {
	rows, err := a.Inbound.List()
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, rows)
}

// InboundRates returns [{id, upRate, downRate}] in bytes/sec from in-memory samples.
// GET /inbounds/rates
func (a *API) InboundRates(c *gin.Context) {
	rates.SampleFromDB()
	ok(c, rates.All())
}

func (a *API) CreateInbound(c *gin.Context) {
	var in model.Inbound
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, 400, err)
		return
	}
	if err := a.Inbound.Create(&in); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, in)
}

func (a *API) UpdateInbound(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var in model.Inbound
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, 400, err)
		return
	}
	in.ID = uint(id)
	if err := a.Inbound.Update(&in); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, in)
}

func (a *API) DeleteInbound(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.Inbound.Delete(uint(id)); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}

func (a *API) CloneInbound(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	cloned, err := a.Inbound.Clone(uint(id))
	if err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, cloned)
}

func (a *API) DisableInvalidInbounds(c *gin.Context) {
	n, err := a.Inbound.DisableInvalid()
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, gin.H{"count": n})
}

func (a *API) CreateClient(c *gin.Context) {
	var cl model.Client
	if err := c.ShouldBindJSON(&cl); err != nil {
		fail(c, 400, err)
		return
	}
	if err := a.Client.Create(&cl); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, cl)
}

func (a *API) UpdateClient(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cl model.Client
	if err := c.ShouldBindJSON(&cl); err != nil {
		fail(c, 400, err)
		return
	}
	cl.ID = uint(id)
	if err := a.Client.Update(&cl); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, cl)
}

func (a *API) DeleteClient(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.Client.Delete(uint(id)); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}

func (a *API) ResetClientTraffic(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.Client.ResetTraffic(uint(id)); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}

func (a *API) KickClient(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cl model.Client
	if err := database.DB.First(&cl, id).Error; err != nil {
		fail(c, 404, err)
		return
	}
	email := strings.TrimSpace(cl.Email)
	ips, _ := clientonline.ListIPs(email)
	ipCount := len(ips)
	_ = clientonline.ClearIPs(email)
	clientonline.ClearLimitWarning(email)

	note := fmt.Sprintf("[panel] Clear IPs & warn %s (cleared %d IPs)", time.Now().UTC().Format(time.RFC3339), ipCount)
	comment := strings.TrimSpace(cl.Comment)
	if comment == "" {
		comment = note
	} else if !strings.Contains(comment, note) {
		if len(comment)+len(note)+3 > 512 {
			// keep newest note, trim old
			comment = note
		} else {
			comment = comment + " | " + note
		}
	}
	_ = database.DB.Model(&model.Client{}).Where("id = ?", cl.ID).Update("comment", comment).Error
	panellog.Append("client kick: id=%d email=%s cleared_ips=%d", cl.ID, email, ipCount)
	ok(c, gin.H{"email": email, "cleared": ipCount, "comment": comment})
}

func (a *API) ClientsLimitWarnings(c *gin.Context) {
	list := clientonline.ListLimitWarnings()
	if list == nil {
		list = []clientonline.LimitWarning{}
	}
	ok(c, list)
}

func (a *API) BulkAdjustClients(c *gin.Context) {
	var req struct {
		IDs     []uint `json:"ids"`
		AddDays int    `json:"addDays"`
		AddGB   int64  `json:"addGB"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	n, err := a.Client.BulkAdjust(req.IDs, req.AddDays, req.AddGB)
	if err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, gin.H{"count": n})
}

func (a *API) BulkAttachClients(c *gin.Context) {
	var req struct {
		IDs        []uint `json:"ids"`
		InboundIDs []uint `json:"inboundIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	n, err := a.Client.BulkAttach(req.IDs, req.InboundIDs)
	if err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, gin.H{"count": n})
}

func (a *API) BulkDetachClients(c *gin.Context) {
	var req struct {
		IDs        []uint `json:"ids"`
		InboundIDs []uint `json:"inboundIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	n, err := a.Client.BulkDetach(req.IDs, req.InboundIDs)
	if err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, gin.H{"count": n})
}

func (a *API) ClientsOnlines(c *gin.Context) {
	emails, m := clientonline.Snapshot()
	if emails == nil {
		emails = []string{}
	}
	if m == nil {
		m = map[string]int64{}
	}
	ok(c, gin.H{"emails": emails, "map": m})
}

func (a *API) ClientIPs(c *gin.Context) {
	email := c.Param("email")
	rows, err := clientonline.ListIPs(email)
	if err != nil {
		fail(c, 500, err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{"ip": r.IP, "lastSeen": r.LastSeen})
	}
	ok(c, out)
}

func (a *API) ClearClientIPs(c *gin.Context) {
	email := c.Param("email")
	if err := clientonline.ClearIPs(email); err != nil {
		fail(c, 500, err)
		return
	}
	clientonline.ClearLimitWarning(email)
	ok(c, nil)
}

func (a *API) ClientHWIDs(c *gin.Context) {
	email := c.Param("email")
	rows, err := clientonline.ListHWIDs(email)
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, rows)
}

func (a *API) AddClientHWID(c *gin.Context) {
	email := c.Param("email")
	var req struct {
		HWID string `json:"hwid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	row, err := clientonline.AddHWID(email, req.HWID)
	if err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, row)
}

func (a *API) DeleteClientHWID(c *gin.Context) {
	email := c.Param("email")
	id, _ := strconv.Atoi(c.Param("id"))
	if err := clientonline.DeleteHWID(email, uint(id)); err != nil {
		fail(c, 404, err)
		return
	}
	ok(c, nil)
}

func (a *API) ClearClientHWIDs(c *gin.Context) {
	email := c.Param("email")
	if err := clientonline.ClearHWIDs(email); err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, nil)
}

func (a *API) ClientLink(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	host := c.Query("host")
	if host != "" && !security.ValidShareHost(host) {
		fail(c, 400, fmt.Errorf("invalid host"))
		return
	}
	link, err := a.Client.ShareLink(uint(id), host)
	if err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, gin.H{"link": link})
}

// ClientLinks returns all share links (one per host) for a client.
// GET /clients/:id/links
func (a *API) ClientLinks(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	host := c.Query("host")
	if host != "" && !security.ValidShareHost(host) {
		fail(c, 400, fmt.Errorf("invalid host"))
		return
	}
	links, err := a.Client.ShareLinks(uint(id), host)
	if err != nil {
		fail(c, 400, err)
		return
	}
	if links == nil {
		links = []string{}
	}
	ok(c, gin.H{"links": links})
}

func (a *API) ClientQR(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	host := c.Query("host")
	if host != "" && !security.ValidShareHost(host) {
		fail(c, 400, fmt.Errorf("invalid host"))
		return
	}
	link, err := a.Client.ShareLink(uint(id), host)
	if err != nil {
		fail(c, 400, err)
		return
	}
	// QR encodes the first link when multiple hosts are configured.
	if i := strings.IndexByte(link, '\n'); i >= 0 {
		link = link[:i]
	}
	png, err := qrcode.Encode(link, qrcode.Medium, 256)
	if err != nil {
		fail(c, 500, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/png", png)
}

func (a *API) ClientSub(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cl model.Client
	if err := database.DB.First(&cl, id).Error; err != nil {
		fail(c, 404, fmt.Errorf("client not found"))
		return
	}
	if !sub.ValidSubID(cl.SubID) {
		fail(c, 400, fmt.Errorf("client has no subscription id"))
		return
	}
	ok(c, gin.H{
		"subId":   cl.SubID,
		"enable":  sub.Enabled(),
		"baseUrl": sub.PublicBaseURL(""),
		"urls":    sub.ClientSubURLs(cl.SubID),
	})
}

func (a *API) SubscriptionInfo(c *gin.Context) {
	formats := []string{"auto", "base64", "singbox"}
	if sub.SubClashEnabled() {
		formats = append(formats, "clash")
	}
	if sub.SubJsonEnabled() {
		formats = append(formats, "json")
	}
	ok(c, gin.H{
		"enable":  sub.Enabled(),
		"subPort": database.GetSetting("subPort"),
		"subPath": sub.NormalizePath(database.GetSetting("subPath")),
		"subHost": database.GetSetting("subHost"),
		"baseUrl": sub.PublicBaseURL(""),
		"formats": formats,
	})
}

func (a *API) GetSettings(c *gin.Context) {
	m, err := database.AllSettingsPublic()
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, m)
}

func (a *API) UpdateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	for k, v := range req {
		if !database.AllowedSettingKeys[k] {
			fail(c, 400, fmt.Errorf("setting %q is not writable via API", k))
			return
		}
		// Keep existing secret when UI sends masked placeholder.
		if database.SensitiveSettings[k] && (v == "***" || strings.Contains(v, "…") || v == "") {
			if v == "" && database.GetSetting(k) != "" && (k == "tgBotToken" || k == "smtpPass" || k == "discordWebhook") {
				continue // blank = leave unchanged when secret already set
			}
			if v == "***" || strings.Contains(v, "…") {
				continue
			}
		}
		maxLen := 8192
		if k == "xrayTemplate" {
			maxLen = 2 << 20 // 2 MiB
		}
		if len(v) > maxLen {
			fail(c, 400, fmt.Errorf("setting %q too large", k))
			return
		}
		if err := sub.ValidateSettings(k, v); err != nil {
			fail(c, 400, err)
			return
		}
		if err := database.SetSetting(k, v); err != nil {
			fail(c, 500, err)
			return
		}
	}
	ok(c, req)
}

func (a *API) ServerStatus(c *gin.Context) {
	var cpuPct float64
	if pcts, err := cpu.Percent(0, false); err == nil && len(pcts) > 0 {
		cpuPct = pcts[0]
	}
	vm, _ := mem.VirtualMemory()
	memPct := 0.0
	if vm != nil {
		memPct = vm.UsedPercent
	}
	tcpCount, udpCount := 0, 0
	if conns, err := psnet.Connections("tcp"); err == nil {
		tcpCount = len(conns)
	}
	if conns, err := psnet.Connections("udp"); err == nil {
		udpCount = len(conns)
	}
var xrayUptime int64
	xrayRunning := a.Xray != nil && a.Xray.IsRunning()
	if a.Xray != nil && xrayRunning {
		xrayUptime = int64(a.Xray.Uptime().Seconds())
	}
	// Refresh inbound traffic rates on every status poll (Dashboard interval).
	rates.SampleFromDB()
	ok(c, gin.H{
		"version":     config.Version,
		"xrayRunning": xrayRunning,
		"cpu":         cpuPct,
		"memory":      memPct,
		"tcpCount":    tcpCount,
		"udpCount":    udpCount,
		"xrayUptime":  xrayUptime,
		"goroutines":  goruntime.NumGoroutine(),
		"extra":       a.RT.Local.Extra.Status(),
	})
}

// PanelLogs returns the last N lines from {DataDir}/panel.log.
// GET /logs/panel?lines=200
func (a *API) PanelLogs(c *gin.Context) {
	lines := 200
	if v := c.Query("lines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			lines = n
		}
	}
	if lines <= 0 {
		lines = 200
	}
	if lines > 2000 {
		lines = 2000
	}
	path := panellog.Path()
	if path == "" {
		ok(c, gin.H{
			"source": "panel",
			"lines":  []string{"panel.log not configured — call panellog.Init(dataDir) on startup"},
			"path":   "",
			"hint":   "panel syslog writes to {dataDir}/panel.log on reload errors and important events",
		})
		return
	}
	out, err := xray.TailFile(path, lines)
	if err != nil {
		fail(c, 500, err)
		return
	}
	if len(out) == 0 {
		out = []string{"(empty or missing) panel.log — important events are appended on xray reload errors"}
	}
	ok(c, gin.H{"source": "panel", "lines": out, "path": path})
}

func (a *API) XrayConfig(c *gin.Context) {
	cfg, err := a.Xray.GenerateConfig()
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, cfg)
}

// XrayConfigIssues lists enabled inbounds skipped during config generation.
func (a *API) XrayConfigIssues(c *gin.Context) {
	issues, err := a.Xray.ConfigIssues()
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, gin.H{"issues": issues})
}

// XrayLogs returns the last N lines from access/error/process log files.
// GET /xray/logs?source=error|access|process&lines=200
func (a *API) XrayLogs(c *gin.Context) {
	source := strings.ToLower(strings.TrimSpace(c.DefaultQuery("source", "error")))
	lines := 200
	if v := c.Query("lines"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			lines = n
		}
	}
	if lines <= 0 {
		lines = 200
	}
	if lines > 2000 {
		lines = 2000
	}
	var path string
	switch source {
	case "error":
		path = a.Xray.ErrorLogPath()
	case "access":
		path = a.Xray.AccessLogPath()
	case "process":
		path = a.Xray.ProcessLogPath()
	default:
		fail(c, 400, fmt.Errorf("source must be error, access, or process"))
		return
	}
	out, err := xray.TailFile(path, lines)
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, gin.H{"source": source, "lines": out, "path": path})
}

func (a *API) XrayRestart(c *gin.Context) {
	if err := a.RT.ReloadLocal(); err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, gin.H{"running": a.Xray.IsRunning()})
}

func (a *API) GetXrayTemplate(c *gin.Context) {
	ok(c, xray.LoadTemplate())
}

func (a *API) GetXrayTemplateDefault(c *gin.Context) {
	ok(c, xray.DefaultTemplate())
}

func (a *API) SetXrayTemplate(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil {
		fail(c, 400, err)
		return
	}
	if len(raw) > 2<<20 {
		fail(c, 400, fmt.Errorf("template too large (max 2MB)"))
		return
	}
	tpl, err := xray.ValidateTemplateJSON(raw)
	if err != nil {
		fail(c, 400, err)
		return
	}
	b, err := json.Marshal(tpl)
	if err != nil {
		fail(c, 500, err)
		return
	}
	if err := database.SetSetting("xrayTemplate", string(b)); err != nil {
		fail(c, 500, err)
		return
	}
	_ = a.RT.ReloadLocal()
	ok(c, tpl)
}

func (a *API) XrayRouteTest(c *gin.Context) {
	var in xray.RouteTestInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, 400, err)
		return
	}
	res, err := a.Xray.RouteTest(in)
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, res)
}

func (a *API) ListOutbounds(c *gin.Context) {
	rows, err := a.Outbound.List()
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, rows)
}

func (a *API) CreateOutbound(c *gin.Context) {
	var o model.Outbound
	if err := c.ShouldBindJSON(&o); err != nil {
		fail(c, 400, err)
		return
	}
	if err := a.Outbound.Create(&o); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, o)
}

func (a *API) UpdateOutbound(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var o model.Outbound
	if err := c.ShouldBindJSON(&o); err != nil {
		fail(c, 400, err)
		return
	}
	o.ID = uint(id)
	if err := a.Outbound.Update(&o); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, o)
}

func (a *API) DeleteOutbound(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.Outbound.Delete(uint(id)); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}

func (a *API) ListNodes(c *gin.Context) {
	rows, err := a.Node.List()
	if err != nil {
		fail(c, 500, err)
		return
	}
	type nodeDTO struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		URL      string `json:"url"`
		Token    string `json:"token"`
		TLSMode  string `json:"tlsMode"`
		Region   string `json:"region"`
		Enable   bool   `json:"enable"`
		Online   bool   `json:"online"`
		LastSeen int64  `json:"lastSeen"`
	}
	out := make([]nodeDTO, 0, len(rows))
	for _, n := range rows {
		out = append(out, nodeDTO{
			ID: n.ID, Name: n.Name, URL: n.URL, Token: security.MaskSecret(n.Token),
			TLSMode: n.TLSMode, Region: n.Region, Enable: n.Enable,
			Online: n.Online, LastSeen: n.LastSeen,
		})
	}
	ok(c, out)
}

func (a *API) CreateNode(c *gin.Context) {
	var n model.Node
	if err := c.ShouldBindJSON(&n); err != nil {
		fail(c, 400, err)
		return
	}
	if err := security.ValidateNodeURL(n.URL); err != nil {
		fail(c, 400, err)
		return
	}
	if err := a.Node.Create(&n); err != nil {
		fail(c, 400, err)
		return
	}
	// one-time full token for initial provisioning
	ok(c, n)
}

func (a *API) UpdateNode(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var n model.Node
	if err := c.ShouldBindJSON(&n); err != nil {
		fail(c, 400, err)
		return
	}
	n.ID = uint(id)
	if err := security.ValidateNodeURL(n.URL); err != nil {
		fail(c, 400, err)
		return
	}
	// empty token in update means "keep existing" when UI sent masked value
	if strings.Contains(n.Token, "…") || n.Token == "***" || n.Token == "" {
		var old model.Node
		if err := database.DB.First(&old, n.ID).Error; err == nil {
			n.Token = old.Token
		}
	}
	if err := a.Node.Update(&n); err != nil {
		fail(c, 400, err)
		return
	}
	n.Token = security.MaskSecret(n.Token)
	ok(c, n)
}

func (a *API) DeleteNode(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.Node.Delete(uint(id)); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}

func (a *API) PingNodes(c *gin.Context) {
	a.RT.PingNodes()
	a.ListNodes(c) // reuse masked DTO — never echo full node tokens
}

// NodeHistory returns in-memory online/latency rings for a node.
// GET /nodes/:id/history
func (a *API) NodeHistory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		fail(c, 400, fmt.Errorf("invalid id"))
		return
	}
	ok(c, nodehist.Get(uint(id)))
}

func (a *API) ListBridges(c *gin.Context) {
	rows, err := a.Bridge.List()
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, rows)
}

func (a *API) CreateBridge(c *gin.Context) {
	var b model.Bridge
	if err := c.ShouldBindJSON(&b); err != nil {
		fail(c, 400, err)
		return
	}
	if err := a.Bridge.Create(&b); err != nil {
		fail(c, 400, err)
		return
	}
	_ = a.RT.ReloadLocal()
	ok(c, b)
}

func (a *API) UpdateBridge(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var b model.Bridge
	if err := c.ShouldBindJSON(&b); err != nil {
		fail(c, 400, err)
		return
	}
	b.ID = uint(id)
	if err := a.Bridge.Update(&b); err != nil {
		fail(c, 400, err)
		return
	}
	_ = a.RT.ReloadLocal()
	ok(c, b)
}

func (a *API) DeleteBridge(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.Bridge.Delete(uint(id)); err != nil {
		fail(c, 400, err)
		return
	}
	_ = a.RT.ReloadLocal()
	ok(c, nil)
}

func (a *API) BridgeHint(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	b, err := a.Bridge.Get(uint(id))
	if err != nil {
		fail(c, 404, err)
		return
	}
	ok(c, a.Bridge.BuildDialerHint(b))
}

func (a *API) ListTgProxy(c *gin.Context) {
	rows, err := a.TgProxy.List()
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, rows)
}

func (a *API) CreateTgProxy(c *gin.Context) {
	var p model.TgProxyProfile
	if err := c.ShouldBindJSON(&p); err != nil {
		fail(c, 400, err)
		return
	}
	if err := a.TgProxy.Create(&p); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, p)
}

func (a *API) UpdateTgProxy(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var p model.TgProxyProfile
	if err := c.ShouldBindJSON(&p); err != nil {
		fail(c, 400, err)
		return
	}
	p.ID = uint(id)
	if err := a.TgProxy.Update(&p); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, p)
}

func (a *API) DeleteTgProxy(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.TgProxy.Delete(uint(id)); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}

func (a *API) StartTgProxy(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.TgProxy.Start(uint(id)); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, a.TgProxy.Status(uint(id)))
}

func (a *API) StopTgProxy(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.TgProxy.Stop(uint(id)); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, a.TgProxy.Status(uint(id)))
}

func (a *API) ListRouting(c *gin.Context) {
	rows, err := a.Routing.List()
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, rows)
}

func (a *API) CreateRouting(c *gin.Context) {
	var r model.RoutingRule
	if err := c.ShouldBindJSON(&r); err != nil {
		fail(c, 400, err)
		return
	}
	if err := a.Routing.Create(&r); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, r)
}

func (a *API) UpdateRouting(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var r model.RoutingRule
	if err := c.ShouldBindJSON(&r); err != nil {
		fail(c, 400, err)
		return
	}
	r.ID = uint(id)
	if err := a.Routing.Update(&r); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, r)
}

func (a *API) DeleteRouting(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.Routing.Delete(uint(id)); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}

func (a *API) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=8,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	sess := sessions.Default(c)
	username, _ := sess.Get("username").(string)
	if username == "" {
		fail(c, 401, fmt.Errorf("unauthorized"))
		return
	}
	if err := a.Auth.ChangePassword(username, req.OldPassword, req.NewPassword); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}

// --- Client Groups ---

func (a *API) ListClientGroups(c *gin.Context) {
	rows, err := a.Client.ListGroups()
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, rows)
}

func (a *API) CreateClientGroup(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	if err := a.Client.CreateGroup(req.Name); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}

func (a *API) RenameClientGroup(c *gin.Context) {
	var req struct {
		OldName string `json:"oldName"`
		NewName string `json:"newName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	n, err := a.Client.RenameGroup(req.OldName, req.NewName)
	if err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, gin.H{"affected": n})
}

func (a *API) DeleteClientGroup(c *gin.Context) {
	name := c.Param("name")
	n, err := a.Client.DeleteGroup(name)
	if err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, gin.H{"affected": n})
}

func (a *API) AssignClientGroup(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		IDs  []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	n, err := a.Client.AssignGroup(req.Name, req.IDs)
	if err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, gin.H{"affected": n})
}

func (a *API) UnassignClientGroup(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	n, err := a.Client.UnassignGroup(req.IDs)
	if err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, gin.H{"affected": n})
}

func (a *API) ResetClientGroupTraffic(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	n, err := a.Client.ResetGroupTraffic(req.Name)
	if err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, gin.H{"affected": n})
}

// --- Subscription Hosts ---

func (a *API) ListHosts(c *gin.Context) {
	rows, err := a.Host.List()
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, rows)
}

func (a *API) CreateHost(c *gin.Context) {
	var h model.Host
	if err := c.ShouldBindJSON(&h); err != nil {
		fail(c, 400, err)
		return
	}
	if err := a.Host.Create(&h); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, h)
}

func (a *API) UpdateHost(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var h model.Host
	if err := c.ShouldBindJSON(&h); err != nil {
		fail(c, 400, err)
		return
	}
	h.ID = uint(id)
	if err := a.Host.Update(&h); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, h)
}

func (a *API) DeleteHost(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := a.Host.Delete(uint(id)); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}

func (a *API) EnableHost(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Enable *bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	enable := true
	if req.Enable != nil {
		enable = *req.Enable
	}
	if err := a.Host.SetEnable(uint(id), enable); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}
