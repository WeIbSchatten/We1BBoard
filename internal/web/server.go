package web

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/we1bboard/we1bboard/internal/bridge"
	"github.com/we1bboard/we1bboard/internal/config"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/extra"
	"github.com/we1bboard/we1bboard/internal/sub"
	"github.com/we1bboard/we1bboard/internal/tgproxy"
	"github.com/we1bboard/we1bboard/internal/web/controller"
	"github.com/we1bboard/we1bboard/internal/web/middleware"
	"github.com/we1bboard/we1bboard/internal/web/runtime"
	"github.com/we1bboard/we1bboard/internal/web/service"
	"github.com/we1bboard/we1bboard/internal/web/tlsutil"
	"github.com/we1bboard/we1bboard/internal/xray"
)

//go:embed all:dist
var distFS embed.FS

type Server struct {
	Cfg     *config.Config
	Xray    *xray.Manager
	Extra   *extra.Manager
	TgProxy *tgproxy.Manager
	RT      *runtime.Hub
	engine  *gin.Engine
}

func NewServer(cfg *config.Config, xrayMgr *xray.Manager, extraMgr *extra.Manager, tg *tgproxy.Manager) *Server {
	local := &runtime.Local{Xray: xrayMgr, Extra: extraMgr, TgProxy: tg}
	hub := runtime.NewHub(local)
	return &Server{Cfg: cfg, Xray: xrayMgr, Extra: extraMgr, TgProxy: tg, RT: hub}
}

func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	middleware.TrustProxy(r)
	r.Use(gin.Recovery(), gin.Logger(), securityHeaders(), middleware.MaxBodyBytes(2<<20))

	secret := database.GetSetting("secret")
	if len(secret) < 32 {
		return fmt.Errorf("session secret missing or too short; check DB setting 'secret'")
	}
	store := cookie.NewStore([]byte(secret))
	store.Options(sessions.Options{
		Path:     "/",
		HttpOnly: true,
		MaxAge:   7 * 24 * 3600,
		SameSite: http.SameSiteLaxMode,
	})
	r.Use(sessions.Sessions("we1b", store))
	r.Use(middleware.CSRFOriginCheck())

	api := &controller.API{
		Auth:     &service.AuthService{},
		Inbound:  &service.InboundService{RT: s.RT},
		Client:   &service.ClientService{RT: s.RT},
		Outbound: &service.OutboundService{RT: s.RT},
		Node:     &service.NodeService{},
		Routing:  &service.RoutingService{RT: s.RT},
		Bridge:   bridge.NewService(),
		TgProxy:  s.TgProxy,
		Xray:     s.Xray,
		RT:       s.RT,
		Cfg:      s.Cfg,
	}

	panelPath := database.GetSetting("panelPath")
	if panelPath == "" {
		panelPath = "/we1b/"
	}
	if !strings.HasPrefix(panelPath, "/") {
		panelPath = "/" + panelPath
	}
	if !strings.HasSuffix(panelPath, "/") {
		panelPath += "/"
	}

	public := r.Group(panelPath + "api")
	{
		public.POST("/login", middleware.LoginRateLimit(), api.Login)
	}

	auth := r.Group(panelPath + "api")
	auth.Use(api.RequireAuth())
	{
		// Node token may only hit these (remote master heartbeat / reload)
		auth.GET("/server/status", api.ServerStatus)
		auth.POST("/xray/restart", api.XrayRestart)

		// Everything else requires interactive admin session
		sess := auth.Group("")
		sess.Use(api.RequireSession())
		{
			sess.POST("/logout", api.Logout)
			sess.GET("/me", api.Me)
			sess.POST("/password", api.ChangePassword)
			sess.GET("/protocols", api.ListProtocols)

			sess.GET("/inbounds", api.ListInbounds)
			sess.POST("/inbounds", api.CreateInbound)
			sess.PUT("/inbounds/:id", api.UpdateInbound)
			sess.DELETE("/inbounds/:id", api.DeleteInbound)

			sess.POST("/clients", api.CreateClient)
			sess.PUT("/clients/:id", api.UpdateClient)
			sess.DELETE("/clients/:id", api.DeleteClient)
			sess.GET("/clients/:id/link", api.ClientLink)
			sess.GET("/clients/:id/qr", api.ClientQR)

			sess.GET("/outbounds", api.ListOutbounds)
			sess.POST("/outbounds", api.CreateOutbound)
			sess.PUT("/outbounds/:id", api.UpdateOutbound)
			sess.DELETE("/outbounds/:id", api.DeleteOutbound)

			sess.GET("/nodes", api.ListNodes)
			sess.POST("/nodes", api.CreateNode)
			sess.PUT("/nodes/:id", api.UpdateNode)
			sess.DELETE("/nodes/:id", api.DeleteNode)
			sess.POST("/nodes/ping", api.PingNodes)

			sess.GET("/bridges", api.ListBridges)
			sess.POST("/bridges", api.CreateBridge)
			sess.PUT("/bridges/:id", api.UpdateBridge)
			sess.DELETE("/bridges/:id", api.DeleteBridge)
			sess.GET("/bridges/:id/hint", api.BridgeHint)

			sess.GET("/tgproxy", api.ListTgProxy)
			sess.POST("/tgproxy", api.CreateTgProxy)
			sess.PUT("/tgproxy/:id", api.UpdateTgProxy)
			sess.DELETE("/tgproxy/:id", api.DeleteTgProxy)
			sess.POST("/tgproxy/:id/start", api.StartTgProxy)
			sess.POST("/tgproxy/:id/stop", api.StopTgProxy)

			sess.GET("/routing", api.ListRouting)
			sess.POST("/routing", api.CreateRouting)
			sess.DELETE("/routing/:id", api.DeleteRouting)

			sess.GET("/settings", api.GetSettings)
			sess.POST("/settings", api.UpdateSettings)

			sess.GET("/xray/config", api.XrayConfig)
		}
	}

	// Remote-node compatibility alias (safe path join, no ..)
	r.Any("/panel/api/*filepath", func(c *gin.Context) {
		p := c.Param("filepath")
		clean := path.Clean("/" + strings.TrimPrefix(p, "/"))
		if strings.Contains(clean, "..") {
			c.AbortWithStatus(400)
			return
		}
		target := panelPath + "api" + clean
		c.Request.URL.Path = target
		r.HandleContext(c)
	})

	subPath := database.GetSetting("subPath")
	if subPath == "" {
		subPath = "/sub/"
	}
	sub.New().Mount(r, subPath)

	static, err := fs.Sub(distFS, "dist")
	if err != nil {
		return fmt.Errorf("embed dist: %w", err)
	}
	fileServer := http.FileServer(http.FS(static))
	r.GET(panelPath+"assets/*filepath", func(c *gin.Context) {
		fp := path.Clean("/" + c.Param("filepath"))
		if strings.Contains(fp, "..") {
			c.AbortWithStatus(400)
			return
		}
		c.Request.URL.Path = "/assets" + fp
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
	r.GET(panelPath, func(c *gin.Context) {
		data, err := distFS.ReadFile("dist/index.html")
		if err != nil {
			c.String(500, "ui missing")
			return
		}
		c.Data(200, "text/html; charset=utf-8", data)
	})
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, panelPath) {
			data, err := distFS.ReadFile("dist/index.html")
			if err != nil {
				c.String(404, "not found")
				return
			}
			c.Data(200, "text/html; charset=utf-8", data)
			return
		}
		c.String(404, "not found")
	})

	s.engine = r
	listen := database.GetSetting("webListen")
	port := database.GetSetting("panelPort")
	certFile := database.GetSetting("certFile")
	keyFile := database.GetSetting("keyFile")
	addr := s.Cfg.Listen
	if addr == "" {
		addr = fmt.Sprintf("%s:%s", listen, port)
	}
	scheme := "http"
	if certFile != "" && keyFile != "" {
		scheme = "https"
	}
	fmt.Printf("We1BBoard listening on %s://%s%s (db=%s)\n", scheme, addr, panelPath, config.GetDBKind())
	return tlsutil.Listen(addr, certFile, keyFile, r)
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; script-src 'self'; connect-src 'self'")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Next()
	}
}
