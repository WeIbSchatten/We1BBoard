package sub

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/protocol"
	"github.com/we1bboard/we1bboard/internal/security"
	"github.com/we1bboard/we1bboard/internal/web/tlsutil"
	"gopkg.in/yaml.v3"
)

// Server serves client subscription endpoints (3x-ui style).
type Server struct {
	Host string
}

func New() *Server { return &Server{} }

// Enabled reports whether the subscription listener should run.
func Enabled() bool {
	v := strings.ToLower(strings.TrimSpace(database.GetSetting("subEnable")))
	return v == "" || v == "true" || v == "1" || v == "yes" || v == "on"
}

// NormalizePath ensures subPath starts and ends with /.
func NormalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		p = "/sub/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// ValidSubID accepts opaque subscription tokens (hex/uuid-like).
// Minimum 16 chars (~64+ bits) so public capability URLs are not guessable.
func ValidSubID(id string) bool {
	if len(id) < 16 || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

// Mount registers subscription routes on an existing engine (same-port mode).
func (s *Server) Mount(r gin.IRouter, basePath string) {
	basePath = strings.TrimSuffix(NormalizePath(basePath), "/")
	g := r.Group(basePath)
	g.Use(subSecurityHeaders(), subRateLimit(), requireSubEnabled())
	g.GET("/:subId", s.handleAuto)
	g.GET("/:subId/json", s.handleJSON)
	g.GET("/:subId/clash", s.handleClash)
	g.GET("/:subId/singbox", s.handleSingBox)
	g.GET("/:subId/qr", s.handleQR)
}

// StartDedicated listens on subPort with only subscription routes.
func (s *Server) StartDedicated(listen, port, certFile, keyFile, basePath string) error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	s.Mount(r, basePath)
	addr := fmt.Sprintf("%s:%s", listen, port)
	if certFile == "" || keyFile == "" {
		fmt.Printf("WARNING: subscription TLS disabled — proxy credentials will be served over cleartext HTTP on %s%s (set certFile/keyFile or put a reverse proxy with TLS in front)\n", addr, NormalizePath(basePath))
	} else {
		fmt.Printf("We1BBoard subscription listening on %s%s (TLS)\n", addr, NormalizePath(basePath))
	}
	return tlsutil.Listen(addr, certFile, keyFile, r)
}

func (s *Server) host() string {
	if s.Host != "" {
		return s.Host
	}
	h := database.GetSetting("subHost")
	if h != "" {
		return h
	}
	return "127.0.0.1"
}

type subEntry struct {
	Client  model.Client
	Inbound model.Inbound
	Link    string
	Name    string
}

func (s *Server) resolve(subID string) ([]subEntry, error) {
	if !ValidSubID(subID) {
		return nil, errNotFound
	}
	var clients []model.Client
	if err := database.DB.Where("sub_id = ? AND enable = ?", subID, true).Find(&clients).Error; err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	clients = filterActiveClients(clients, now)
	if len(clients) == 0 {
		return nil, nil
	}
	ids := map[uint]bool{}
	for _, c := range clients {
		for _, iid := range model.ParseInboundIDList(&c) {
			ids[iid] = true
		}
	}
	idList := make([]uint, 0, len(ids))
	for id := range ids {
		idList = append(idList, id)
	}
	var inbounds []model.Inbound
	if err := database.DB.Where("id IN ? AND enable = ?", idList, true).Find(&inbounds).Error; err != nil {
		return nil, err
	}
	imap := map[uint]model.Inbound{}
	for _, in := range inbounds {
		if inboundActive(in, now) {
			imap[in.ID] = in
		}
	}
	host := s.host()
	out := make([]subEntry, 0, len(clients)*2)
	seenExtra := map[string]bool{}
	hosts, _ := loadEnabledHosts()
	for _, cl := range clients {
		for _, iid := range model.ParseInboundIDList(&cl) {
			in, ok := imap[iid]
			if !ok {
				continue
			}
			links, err := protocol.ShareLinksForHosts(&in, cl, host, hosts)
			if err != nil || len(links) == 0 {
				continue
			}
			baseName := cl.Email
			if baseName == "" {
				baseName = fmt.Sprintf("%s-%d", in.Protocol, cl.ID)
			}
			if len(model.ParseInboundIDList(&cl)) > 1 {
				baseName = fmt.Sprintf("%s [%s]", baseName, in.Remark)
				if in.Remark == "" {
					baseName = fmt.Sprintf("%s [%s:%d]", cl.Email, in.Protocol, in.Port)
				}
			}
			for i, link := range links {
				name := baseName
				if len(links) > 1 {
					name = fmt.Sprintf("%s #%d", baseName, i+1)
				}
				out = append(out, subEntry{Client: cl, Inbound: in, Link: link, Name: name})
			}
		}
		for _, extra := range model.ExtraLinkLines(cl.ExtraLinks) {
			if seenExtra[extra] {
				continue
			}
			seenExtra[extra] = true
			out = append(out, subEntry{
				Client:  cl,
				Inbound: model.Inbound{Protocol: "extra", Remark: "extra"},
				Link:    extra,
				Name:    "extra",
			})
		}
	}
	return out, nil
}

func loadEnabledHosts() ([]model.Host, error) {
	var rows []model.Host
	err := database.DB.Where("enable = ?", true).Order("sort_order asc, id asc").Find(&rows).Error
	return rows, err
}

var errNotFound = fmt.Errorf("not found")

func filterActiveClients(clients []model.Client, nowMs int64) []model.Client {
	out := make([]model.Client, 0, len(clients))
	for _, c := range clients {
		if !clientActive(c, nowMs) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func clientActive(c model.Client, nowMs int64) bool {
	if !c.Enable {
		return false
	}
	if c.ExpiryTime > 0 && c.ExpiryTime < nowMs {
		return false
	}
	if c.TotalGB > 0 {
		limit := c.TotalGB * 1024 * 1024 * 1024
		if c.Up+c.Down >= limit {
			return false
		}
	}
	return true
}

func inboundActive(in model.Inbound, nowMs int64) bool {
	if !in.Enable {
		return false
	}
	if in.ExpiryTime > 0 && in.ExpiryTime < nowMs {
		return false
	}
	if in.Total > 0 && in.Up+in.Down >= in.Total {
		return false
	}
	return true
}

func (s *Server) writeUserinfo(c *gin.Context, entries []subEntry) {
	var up, down, total int64
	var expire int64
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
	}
	c.Header("Subscription-Userinfo", fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", up, down, total, expire))
	c.Header("Profile-Update-Interval", "24")
	title := database.GetSetting("subTitle")
	if title == "" {
		title = "We1BBoard"
	}
	c.Header("Profile-Title", "base64:"+base64.StdEncoding.EncodeToString([]byte(title)))
	c.Header("Content-Disposition", `attachment; filename="we1bboard"`)
}

func detectFormat(ua, formatQ string) string {
	f := strings.ToLower(strings.TrimSpace(formatQ))
	switch f {
	case "clash", "singbox", "sing-box", "json", "base64", "raw":
		if f == "sing-box" {
			return "singbox"
		}
		if f == "raw" {
			return "base64"
		}
		return f
	}
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "clash") || strings.Contains(ua, "stash") ||
		strings.Contains(ua, "mihomo") || strings.Contains(ua, "nuko") ||
		strings.Contains(ua, "surge"):
		if SubClashEnabled() {
			return "clash"
		}
		return "base64"
	case strings.Contains(ua, "sing-box") || strings.Contains(ua, "singbox") ||
		strings.Contains(ua, "sfa") || strings.Contains(ua, "sfi") ||
		strings.Contains(ua, "sfm") || strings.Contains(ua, "sft"):
		return "singbox"
	default:
		return "base64"
	}
}

// SubJsonEnabled reports whether JSON subscription format is allowed.
func SubJsonEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(database.GetSetting("subJsonEnable")))
	return v == "" || v == "true" || v == "1" || v == "yes" || v == "on"
}

// SubClashEnabled reports whether Clash subscription format is allowed.
func SubClashEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(database.GetSetting("subClashEnable")))
	return v == "" || v == "true" || v == "1" || v == "yes" || v == "on"
}

func (s *Server) handleAuto(c *gin.Context) {
	subID := c.Param("subId")
	if strings.EqualFold(c.Query("format"), "info") {
		s.serveInfoJSON(c, subID)
		return
	}
	if s.maybeServeSubPage(c, subID) {
		return
	}
	entries, err := s.resolve(subID)
	if err != nil {
		if err == errNotFound {
			c.Status(http.StatusNotFound)
			return
		}
		c.String(http.StatusInternalServerError, "error")
		return
	}
	s.writeUserinfo(c, entries)
	switch detectFormat(c.GetHeader("User-Agent"), c.Query("format")) {
	case "clash":
		if !SubClashEnabled() {
			c.Status(http.StatusNotFound)
			return
		}
		s.respondClash(c, entries)
	case "singbox":
		s.respondSingBox(c, entries)
	case "json":
		if !SubJsonEnabled() {
			c.Status(http.StatusNotFound)
			return
		}
		s.respondJSON(c, entries)
	default:
		s.respondBase64(c, entries)
	}
}

func (s *Server) handleJSON(c *gin.Context) {
	if !SubJsonEnabled() {
		c.Status(http.StatusNotFound)
		return
	}
	subID := c.Param("subId")
	if s.maybeServeSubPage(c, subID) {
		return
	}
	entries, err := s.resolve(subID)
	if err != nil {
		if err == errNotFound {
			c.Status(http.StatusNotFound)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error"})
		return
	}
	s.writeUserinfo(c, entries)
	s.respondJSON(c, entries)
}

func (s *Server) handleClash(c *gin.Context) {
	if !SubClashEnabled() {
		c.Status(http.StatusNotFound)
		return
	}
	subID := c.Param("subId")
	if s.maybeServeSubPage(c, subID) {
		return
	}
	entries, err := s.resolve(subID)
	if err != nil {
		if err == errNotFound {
			c.Status(http.StatusNotFound)
			return
		}
		c.String(http.StatusInternalServerError, "error")
		return
	}
	s.writeUserinfo(c, entries)
	s.respondClash(c, entries)
}

func (s *Server) handleSingBox(c *gin.Context) {
	subID := c.Param("subId")
	if s.maybeServeSubPage(c, subID) {
		return
	}
	entries, err := s.resolve(subID)
	if err != nil {
		if err == errNotFound {
			c.Status(http.StatusNotFound)
			return
		}
		c.String(http.StatusInternalServerError, "error")
		return
	}
	s.writeUserinfo(c, entries)
	s.respondSingBox(c, entries)
}

func (s *Server) respondBase64(c *gin.Context, entries []subEntry) {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, e.Link)
	}
	body := base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n")))
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, body)
}

func (s *Server) respondJSON(c *gin.Context, entries []subEntry) {
	type item struct {
		Link     string `json:"link"`
		Protocol string `json:"protocol"`
		Email    string `json:"email"`
		Remark   string `json:"remark"`
	}
	out := make([]item, 0, len(entries))
	for _, e := range entries {
		out = append(out, item{
			Link: e.Link, Protocol: string(e.Inbound.Protocol),
			Email: e.Client.Email, Remark: e.Inbound.Remark,
		})
	}
	c.JSON(http.StatusOK, gin.H{"clients": out})
}

func (s *Server) respondClash(c *gin.Context, entries []subEntry) {
	proxies := []map[string]any{}
	names := []string{}
	for _, e := range entries {
		proxy := clashProxy(&e.Inbound, e.Client, s.host(), e.Name)
		if proxy == nil {
			continue
		}
		proxies = append(proxies, proxy)
		names = append(names, e.Name)
	}
	doc := map[string]any{
		"proxies": proxies,
		"proxy-groups": []map[string]any{
			{"name": "We1BBoard", "type": "select", "proxies": names},
		},
		"rules": []string{"MATCH,We1BBoard"},
	}
	b, _ := yaml.Marshal(doc)
	c.Data(http.StatusOK, "text/yaml; charset=utf-8", b)
}

func (s *Server) respondSingBox(c *gin.Context, entries []subEntry) {
	outbounds := make([]map[string]any, 0, len(entries)+1)
	tags := make([]string, 0, len(entries))
	for _, e := range entries {
		ob := singBoxOutbound(&e.Inbound, e.Client, s.host(), e.Name)
		if ob == nil {
			continue
		}
		outbounds = append(outbounds, ob)
		tags = append(tags, e.Name)
	}
	outbounds = append(outbounds, map[string]any{
		"type": "selector", "tag": "proxy", "outbounds": tags,
	})
	doc := map[string]any{
		"outbounds": outbounds,
	}
	c.JSON(http.StatusOK, doc)
}

func clashProxy(in *model.Inbound, cl model.Client, host, name string) map[string]any {
	stream := protocol.ParseStream(in.StreamSettings)
	network, _ := stream["network"].(string)
	securityMode, _ := stream["security"].(string)
	switch in.Protocol {
	case model.ProtoVLESS:
		p := map[string]any{
			"name": name, "type": "vless", "server": host, "port": in.Port,
			"uuid": cl.UUID, "network": network, "udp": true,
		}
		if cl.Flow != "" {
			p["flow"] = cl.Flow
		}
		if securityMode == "reality" || securityMode == "tls" {
			p["tls"] = true
		}
		if securityMode == "reality" {
			opts := map[string]any{}
			if rs, ok := stream["realitySettings"].(map[string]any); ok {
				if pbk, ok := rs["publicKey"].(string); ok {
					opts["public-key"] = pbk
				}
				if sid, ok := rs["shortIds"].([]any); ok && len(sid) > 0 {
					opts["short-id"] = fmt.Sprint(sid[0])
				}
				if sni, ok := rs["serverNames"].([]any); ok && len(sni) > 0 {
					p["servername"] = fmt.Sprint(sni[0])
				}
				if fp, ok := rs["fingerprint"].(string); ok {
					p["client-fingerprint"] = fp
				}
			}
			p["reality-opts"] = opts
		}
		return p
	case model.ProtoTrojan:
		pw := cl.Password
		if pw == "" {
			pw = cl.UUID
		}
		return map[string]any{
			"name": name, "type": "trojan", "server": host, "port": in.Port,
			"password": pw, "udp": true,
		}
	case model.ProtoShadowsocks:
		settings := protocol.ParseSettings(in.Settings)
		method, _ := settings["method"].(string)
		pw := cl.Password
		if pw == "" {
			pw = cl.UUID
		}
		return map[string]any{
			"name": name, "type": "ss", "server": host, "port": in.Port,
			"cipher": method, "password": pw, "udp": true,
		}
	case model.ProtoVMess:
		return map[string]any{
			"name": name, "type": "vmess", "server": host, "port": in.Port,
			"uuid": cl.UUID, "alterId": 0, "cipher": "auto", "udp": true,
			"network": network,
		}
	default:
		return nil
	}
}

func singBoxOutbound(in *model.Inbound, cl model.Client, host, name string) map[string]any {
	stream := protocol.ParseStream(in.StreamSettings)
	network, _ := stream["network"].(string)
	securityMode, _ := stream["security"].(string)
	switch in.Protocol {
	case model.ProtoVLESS:
		ob := map[string]any{
			"type": "vless", "tag": name,
			"server": host, "server_port": in.Port,
			"uuid": cl.UUID,
		}
		if cl.Flow != "" {
			ob["flow"] = cl.Flow
		}
		if tls := singBoxTLS(stream, securityMode); tls != nil {
			ob["tls"] = tls
		}
		if tr := singBoxTransport(stream, network); tr != nil {
			ob["transport"] = tr
		}
		return ob
	case model.ProtoTrojan:
		pw := cl.Password
		if pw == "" {
			pw = cl.UUID
		}
		ob := map[string]any{
			"type": "trojan", "tag": name,
			"server": host, "server_port": in.Port,
			"password": pw,
		}
		if tls := singBoxTLS(stream, securityMode); tls != nil {
			ob["tls"] = tls
		}
		if tr := singBoxTransport(stream, network); tr != nil {
			ob["transport"] = tr
		}
		return ob
	case model.ProtoShadowsocks:
		settings := protocol.ParseSettings(in.Settings)
		method, _ := settings["method"].(string)
		if method == "" {
			method = "aes-256-gcm"
		}
		pw := cl.Password
		if pw == "" {
			pw = cl.UUID
		}
		return map[string]any{
			"type": "shadowsocks", "tag": name,
			"server": host, "server_port": in.Port,
			"method": method, "password": pw,
		}
	case model.ProtoVMess:
		ob := map[string]any{
			"type": "vmess", "tag": name,
			"server": host, "server_port": in.Port,
			"uuid": cl.UUID, "security": "auto",
		}
		if tls := singBoxTLS(stream, securityMode); tls != nil {
			ob["tls"] = tls
		}
		if tr := singBoxTransport(stream, network); tr != nil {
			ob["transport"] = tr
		}
		return ob
	default:
		return nil
	}
}

func singBoxTLS(stream map[string]any, securityMode string) map[string]any {
	if securityMode != "tls" && securityMode != "reality" {
		return nil
	}
	tls := map[string]any{"enabled": true}
	if securityMode == "reality" {
		tls["reality"] = map[string]any{"enabled": true}
		if rs, ok := stream["realitySettings"].(map[string]any); ok {
			if sni, ok := rs["serverNames"].([]any); ok && len(sni) > 0 {
				tls["server_name"] = fmt.Sprint(sni[0])
			}
			if pbk, ok := rs["publicKey"].(string); ok {
				tls["reality"].(map[string]any)["public_key"] = pbk
			}
			if sid, ok := rs["shortIds"].([]any); ok && len(sid) > 0 {
				tls["reality"].(map[string]any)["short_id"] = fmt.Sprint(sid[0])
			}
			if fp, ok := rs["fingerprint"].(string); ok {
				tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
			}
		}
		return tls
	}
	if ts, ok := stream["tlsSettings"].(map[string]any); ok {
		if sni, ok := ts["serverName"].(string); ok && sni != "" {
			tls["server_name"] = sni
		}
		if fp, ok := ts["fingerprint"].(string); ok && fp != "" {
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
		}
		if alpn, ok := ts["alpn"].([]any); ok && len(alpn) > 0 {
			parts := make([]string, 0, len(alpn))
			for _, a := range alpn {
				parts = append(parts, fmt.Sprint(a))
			}
			tls["alpn"] = parts
		}
	}
	return tls
}

func singBoxTransport(stream map[string]any, network string) map[string]any {
	switch network {
	case "ws":
		tr := map[string]any{"type": "ws"}
		if ws, ok := stream["wsSettings"].(map[string]any); ok {
			if p, ok := ws["path"].(string); ok {
				tr["path"] = p
			}
			if h, ok := ws["headers"].(map[string]any); ok {
				if host, ok := h["Host"].(string); ok {
					tr["headers"] = map[string]any{"Host": host}
				}
			}
		}
		return tr
	case "grpc":
		tr := map[string]any{"type": "grpc"}
		if gs, ok := stream["grpcSettings"].(map[string]any); ok {
			if sn, ok := gs["serviceName"].(string); ok {
				tr["service_name"] = sn
			}
		}
		return tr
	case "httpupgrade":
		tr := map[string]any{"type": "httpupgrade"}
		if hs, ok := stream["httpupgradeSettings"].(map[string]any); ok {
			if p, ok := hs["path"].(string); ok {
				tr["path"] = p
			}
		}
		return tr
	default:
		return nil
	}
}

// PublicBaseURL builds the subscription base URL (without subId).
func PublicBaseURL(_ string) string {
	host := database.GetSetting("subHost")
	if host == "" || !security.ValidShareHost(host) {
		host = "127.0.0.1"
	}
	port := database.GetSetting("subPort")
	if port == "" {
		port = "2096"
	}
	path := NormalizePath(database.GetSetting("subPath"))
	scheme := "http"
	if database.GetSetting("certFile") != "" && database.GetSetting("keyFile") != "" {
		scheme = "https"
	}
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		return fmt.Sprintf("%s://%s%s", scheme, host, path)
	}
	return fmt.Sprintf("%s://%s:%s%s", scheme, host, port, path)
}

// ClientSubURLs returns absolute subscription links for a subId.
func ClientSubURLs(subID string) map[string]string {
	base := strings.TrimSuffix(PublicBaseURL(""), "/") + "/" + subID
	return map[string]string{
		"auto":    base,
		"base64":  base + "?format=base64",
		"clash":   base + "/clash",
		"singbox": base + "/singbox",
		"json":    base + "/json",
	}
}

// ValidateSettings checks subscription-related setting values.
func ValidateSettings(key, value string) error {
	switch key {
	case "subPort":
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("invalid subPort")
		}
	case "subPath":
		p := strings.TrimSpace(value)
		if p == "" || strings.Contains(p, "..") || strings.ContainsAny(p, " \t\r\n") {
			return fmt.Errorf("invalid subPath")
		}
		if !strings.HasPrefix(p, "/") {
			return fmt.Errorf("subPath must start with /")
		}
		norm := NormalizePath(p)
		if norm == "/" {
			return fmt.Errorf("subPath must not be root /")
		}
	case "subEnable":
		v := strings.ToLower(strings.TrimSpace(value))
		switch v {
		case "true", "false", "1", "0", "yes", "no", "on", "off", "":
		default:
			return fmt.Errorf("invalid subEnable")
		}
	case "subJsonEnable", "subClashEnable", "tgBotEnable", "tgNotifyLogin", "tgNotifyTraffic", "twoFactorEnable":
		v := strings.ToLower(strings.TrimSpace(value))
		switch v {
		case "true", "false", "1", "0", "yes", "no", "on", "off", "":
		default:
			return fmt.Errorf("invalid %s", key)
		}
	case "tgBotToken":
		v := strings.TrimSpace(value)
		if v == "" || v == "***" || strings.Contains(v, "…") {
			return nil
		}
		if len(v) > 128 || !strings.Contains(v, ":") {
			return fmt.Errorf("invalid tgBotToken")
		}
		for _, c := range v {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == ':' {
				continue
			}
			return fmt.Errorf("invalid tgBotToken")
		}
	case "tgBotChatId":
		v := strings.TrimSpace(value)
		if v == "" {
			return nil
		}
		if len(v) > 64 {
			return fmt.Errorf("invalid tgBotChatId")
		}
		if strings.HasPrefix(v, "@") {
			name := v[1:]
			if name == "" {
				return fmt.Errorf("invalid tgBotChatId")
			}
			for _, c := range name {
				if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
					continue
				}
				return fmt.Errorf("invalid tgBotChatId")
			}
			return nil
		}
		start := 0
		if v[0] == '-' {
			start = 1
		}
		if start >= len(v) {
			return fmt.Errorf("invalid tgBotChatId")
		}
		for _, c := range v[start:] {
			if c < '0' || c > '9' {
				return fmt.Errorf("invalid tgBotChatId")
			}
		}
	case "subHost":
		if value != "" && !security.ValidShareHost(value) {
			return fmt.Errorf("invalid subHost")
		}
	case "subTitle":
		if len(value) > 128 {
			return fmt.Errorf("subTitle too long")
		}
	case "subSupportUrl":
		v := strings.TrimSpace(value)
		if v == "" {
			return nil
		}
		if len(v) > 512 {
			return fmt.Errorf("subSupportUrl too long")
		}
		if !strings.HasPrefix(v, "https://") && !strings.HasPrefix(v, "http://") {
			return fmt.Errorf("subSupportUrl must be http(s) URL")
		}
		if strings.ContainsAny(v, " \t\r\n\"'<>") {
			return fmt.Errorf("invalid subSupportUrl")
		}
	case "subThemeDir":
		return ValidateThemeDir(value)
	case "subAnnounce":
		if len(value) > 2048 {
			return fmt.Errorf("subAnnounce too long")
		}
	case "panelPath":
		p := NormalizePath(value)
		if p == "/panel/" {
			return fmt.Errorf("panelPath /panel/ conflicts with remote-node API alias")
		}
		if p == "/" {
			return fmt.Errorf("panelPath must not be root /")
		}
	case "ufwEnable":
		v := strings.ToLower(strings.TrimSpace(value))
		switch v {
		case "true", "false", "1", "0", "yes", "no", "on", "off", "":
		default:
			return fmt.Errorf("invalid ufwEnable")
		}
	}
	return nil
}
