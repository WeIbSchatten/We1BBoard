package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/skip2/go-qrcode"
	"github.com/we1bboard/we1bboard/internal/bridge"
	"github.com/we1bboard/we1bboard/internal/config"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/protocol"
	"github.com/we1bboard/we1bboard/internal/security"
	"github.com/we1bboard/we1bboard/internal/sub"
	"github.com/we1bboard/we1bboard/internal/tgproxy"
	"github.com/we1bboard/we1bboard/internal/web/middleware"
	"github.com/we1bboard/we1bboard/internal/web/runtime"
	"github.com/we1bboard/we1bboard/internal/web/service"
	"github.com/we1bboard/we1bboard/internal/xray"
)

type API struct {
	Auth     *service.AuthService
	Inbound  *service.InboundService
	Client   *service.ClientService
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
	ok(c, gin.H{
		"enable":  sub.Enabled(),
		"subPort": database.GetSetting("subPort"),
		"subPath": sub.NormalizePath(database.GetSetting("subPath")),
		"subHost": database.GetSetting("subHost"),
		"baseUrl": sub.PublicBaseURL(""),
		"formats": []string{"auto", "base64", "clash", "singbox", "json"},
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
		if len(v) > 8192 {
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
	ok(c, gin.H{
		"version":     config.Version,
		"xrayRunning": a.Xray != nil && a.Xray.IsRunning(),
		"cpu":         cpuPct,
		"memory":      memPct,
		"extra":       a.RT.Local.Extra.Status(),
	})
}

func (a *API) XrayConfig(c *gin.Context) {
	cfg, err := a.Xray.GenerateConfig()
	if err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, cfg)
}

func (a *API) XrayRestart(c *gin.Context) {
	if err := a.RT.ReloadLocal(); err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, gin.H{"running": a.Xray.IsRunning()})
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
