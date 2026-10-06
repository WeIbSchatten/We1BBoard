package sub

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
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
	g.Use(subSecurityHeaders(), subRateLimit(), requireSubEnabled(), enforceHWID())
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
	return DefaultShareHost("")
}

// DedicatedSubPort reports whether subscription listens on its own TLS port.
func DedicatedSubPort() bool {
	subPort := database.GetSetting("subPort")
	if subPort == "" {
		subPort = "2096"
	}
	panelPort := database.GetSetting("panelPort")
	if panelPort == "" {
		panelPort = "2053"
	}
	certFile := database.GetSetting("certFile")
	keyFile := database.GetSetting("keyFile")
	return subPort != panelPort && certFile != "" && keyFile != ""
}

// EffectiveSubPort is the port clients must use for subscription URLs.
func EffectiveSubPort() string {
	panelPort := database.GetSetting("panelPort")
	if panelPort == "" {
		panelPort = "2053"
	}
	if DedicatedSubPort() {
		subPort := database.GetSetting("subPort")
		if subPort == "" {
			return "2096"
		}
		return subPort
	}
	return panelPort
}

// DefaultShareHost picks subHost, then optional hint (request host), never empty.
func DefaultShareHost(hint string) string {
	h := strings.TrimSpace(database.GetSetting("subHost"))
	if h != "" && security.ValidShareHost(h) {
		return h
	}
	hint = strings.TrimSpace(hint)
	if hint != "" {
		hint = stripHostPort(hint)
		if security.ValidShareHost(hint) {
			return hint
		}
	}
	return "127.0.0.1"
}

func stripHostPort(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return ""
	}
	// bracketed IPv6: [2001:db8::1]:443
	if strings.HasPrefix(hostport, "[") {
		if i := strings.Index(hostport, "]"); i > 0 {
			return hostport[1:i]
		}
	}
	// hostname:port or ipv4:port — only strip if single colon and numeric port
	if host, port, err := splitHostPortSafe(hostport); err == nil && port != "" {
		return host
	}
	return hostport
}

func splitHostPortSafe(hostport string) (host, port string, err error) {
	return net.SplitHostPort(hostport)
}

// PublicBaseURL builds the subscription base URL (without subId).
// hintHost is used when subHost setting is empty (typically the panel request Host).
//
// If setting subURI is set (3x-ui style), it overrides host/port/scheme entirely.
// Examples: https://vpn.example.com:2096  or  https://vpn.example.com/sub/
func PublicBaseURL(hintHost string) string {
	if uri := strings.TrimSpace(database.GetSetting("subURI")); uri != "" {
		return normalizeSubURI(uri)
	}
	host := DefaultShareHost(hintHost)
	port := EffectiveSubPort()
	path := NormalizePath(database.GetSetting("subPath"))
	scheme := "http"
	if database.GetSetting("certFile") != "" && database.GetSetting("keyFile") != "" {
		scheme = "https"
	}
	// Behind reverse proxy without local certs: allow forcing https via setting.
	if v := strings.ToLower(strings.TrimSpace(database.GetSetting("subForceTLS"))); v == "true" || v == "1" || v == "yes" || v == "on" {
		scheme = "https"
	}
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		return fmt.Sprintf("%s://%s%s", scheme, host, path)
	}
	return fmt.Sprintf("%s://%s:%s%s", scheme, host, port, path)
}

func normalizeSubURI(uri string) string {
	uri = strings.TrimSpace(uri)
	if !strings.Contains(uri, "://") {
		uri = "https://" + uri
	}
	u, err := url.Parse(uri)
	if err != nil || u.Host == "" {
		path := NormalizePath(database.GetSetting("subPath"))
		return strings.TrimRight(uri, "/") + path
	}
	path := NormalizePath(database.GetSetting("subPath"))
	// If user already included a path (e.g. /sub/), keep it; else append subPath.
	if u.Path == "" || u.Path == "/" {
		u.Path = strings.TrimSuffix(path, "/")
	}
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	return u.String()
}

// ClientSubURLs returns absolute subscription links for a subId.
func ClientSubURLs(subID string, hintHost string) map[string]string {
	base := strings.TrimSuffix(PublicBaseURL(hintHost), "/") + "/" + subID
	return map[string]string{
		"auto":    base,
		"base64":  base + "?format=base64",
		"clash":   base + "/clash",
		"singbox": base + "/singbox",
		"json":    base + "/json",
	}
}

type subEntry struct {
	Client  model.Client
	Inbound model.Inbound
	Address string
	Link    string
	Name    string
}

func (s *Server) resolve(subID, shareHost string) ([]subEntry, error) {
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
	host := strings.TrimSpace(shareHost)
	if host == "" {
		host = s.host()
	} else {
		host = DefaultShareHost(host)
	}
	out := make([]subEntry, 0, len(clients)*2)
	seenExtra := map[string]bool{}
	hosts, _ := loadEnabledHosts()
	for _, cl := range clients {
		for _, iid := range model.ParseInboundIDList(&cl) {
			in, ok := imap[iid]
			if !ok {
				continue
			}
			results, err := protocol.ShareResultsForHosts(&in, cl, host, hosts)
			if err != nil || len(results) == 0 {
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
			for i, r := range results {
				name := baseName
				if len(results) > 1 {
					name = fmt.Sprintf("%s #%d", baseName, i+1)
				}
				out = append(out, subEntry{
					Client:  cl,
					Inbound: r.Inbound,
					Address: r.Address,
					Link:    r.Link,
					Name:    name,
				})
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
				Address: host,
				Link:    extra,
				Name:    "extra",
			})
		}
	}
	return out, nil
}

// requestShareHost picks address for share links inside a subscription fetch.
func requestShareHost(c *gin.Context) string {
	if h := strings.TrimSpace(c.Query("host")); h != "" && security.ValidShareHost(stripHostPort(h)) {
		return stripHostPort(h)
	}
	return DefaultShareHost(c.Request.Host)
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

	happ := strings.Contains(c.GetHeader("User-Agent"), "Happ") || c.Query("happ") == "1"
	if happ {
		if support := strings.TrimSpace(database.GetSetting("subSupportUrl")); support != "" {
			c.Header("Support-Url", support)
			c.Header("support-url", support)
		}
		c.Header("Content-Disposition", `attachment; filename="happ.txt"`)
	} else {
		c.Header("Content-Disposition", `attachment; filename="we1bboard"`)
	}
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
	entries, err := s.resolve(subID, requestShareHost(c))
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
	entries, err := s.resolve(subID, requestShareHost(c))
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
	entries, err := s.resolve(subID, requestShareHost(c))
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
	entries, err := s.resolve(subID, requestShareHost(c))
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
	if len(entries) == 0 {
		c.Status(http.StatusNotFound)
		return
	}
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, e.Link)
	}
	body := base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n")))
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, body)
}

func (s *Server) respondJSON(c *gin.Context, entries []subEntry) {
	out := make([]map[string]any, 0, len(entries))
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		addr := e.Address
		if addr == "" {
			addr = s.host()
		}
		ob := jsonOutbound(&e.Inbound, e.Client, addr, e.Name, e.Link)
		if ob == nil {
			continue
		}
		out = append(out, ob)
		if tag, _ := ob["tag"].(string); tag != "" {
			names = append(names, tag)
		}
	}
	// Append proxy-group style objects when sub balancers match.
	for _, g := range subBalancerGroups(entries, names) {
		out = append(out, g)
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) respondClash(c *gin.Context, entries []subEntry) {
	proxies := []map[string]any{}
	names := []string{}
	for _, e := range entries {
		addr := e.Address
		if addr == "" {
			addr = s.host()
		}
		proxy := clashProxy(&e.Inbound, e.Client, addr, e.Name)
		if proxy == nil {
			continue
		}
		proxies = append(proxies, proxy)
		names = append(names, e.Name)
	}
	groups := []map[string]any{
		{"name": "We1BBoard", "type": "select", "proxies": names},
	}
	for _, g := range clashBalancerGroups(entries, names) {
		groups = append(groups, g)
	}
	doc := map[string]any{
		"proxies":      proxies,
		"proxy-groups": groups,
		"rules":        []string{"MATCH,We1BBoard"},
	}
	b, _ := yaml.Marshal(doc)
	c.Data(http.StatusOK, "text/yaml; charset=utf-8", b)
}

func (s *Server) respondSingBox(c *gin.Context, entries []subEntry) {
	outbounds := make([]map[string]any, 0, len(entries)+1)
	tags := make([]string, 0, len(entries))
	for _, e := range entries {
		addr := e.Address
		if addr == "" {
			addr = s.host()
		}
		ob := singBoxOutbound(&e.Inbound, e.Client, addr, e.Name)
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
	if in.Protocol == "extra" {
		return nil
	}
	stream := protocol.ParseStream(in.StreamSettings)
	network, _ := stream["network"].(string)
	if network == "" {
		network = "tcp"
	}
	securityMode, _ := stream["security"].(string)
	var p map[string]any
	switch in.Protocol {
	case model.ProtoVLESS:
		p = map[string]any{
			"name": name, "type": "vless", "server": host, "port": in.Port,
			"uuid": cl.UUID, "network": network, "udp": true,
		}
		if cl.Flow != "" {
			p["flow"] = cl.Flow
		}
	case model.ProtoTrojan:
		pw := cl.Password
		if pw == "" {
			pw = cl.UUID
		}
		p = map[string]any{
			"name": name, "type": "trojan", "server": host, "port": in.Port,
			"password": pw, "udp": true, "network": network,
		}
	case model.ProtoShadowsocks:
		settings := protocol.ParseSettings(in.Settings)
		method, _ := settings["method"].(string)
		pw := cl.Password
		if pw == "" {
			pw = cl.UUID
		}
		p = map[string]any{
			"name": name, "type": "ss", "server": host, "port": in.Port,
			"cipher": method, "password": pw, "udp": true,
		}
		return p
	case model.ProtoVMess:
		p = map[string]any{
			"name": name, "type": "vmess", "server": host, "port": in.Port,
			"uuid": cl.UUID, "alterId": 0, "cipher": "auto", "udp": true,
			"network": network,
		}
	default:
		return nil
	}
	applyClashStream(p, stream, network, securityMode)
	return p
}

func applyClashStream(p map[string]any, stream map[string]any, network, securityMode string) {
	if securityMode == "reality" || securityMode == "tls" {
		p["tls"] = true
	}
	if securityMode == "reality" {
		opts := map[string]any{}
		if rs, ok := stream["realitySettings"].(map[string]any); ok {
			if pbk := protocol.ResolveRealityPublicKey(rs); pbk != "" {
				opts["public-key"] = pbk
			}
			if sid, ok := rs["shortIds"].([]any); ok && len(sid) > 0 {
				opts["short-id"] = fmt.Sprint(sid[0])
			} else if sid, ok := rs["shortIds"].([]string); ok && len(sid) > 0 {
				opts["short-id"] = sid[0]
			}
			if sni, ok := rs["serverNames"].([]any); ok && len(sni) > 0 {
				p["servername"] = fmt.Sprint(sni[0])
			}
			if settings, ok := rs["settings"].(map[string]any); ok {
				if sni, _ := settings["serverName"].(string); sni != "" {
					p["servername"] = sni
				}
				if fp, _ := settings["fingerprint"].(string); fp != "" {
					p["client-fingerprint"] = fp
				}
			}
			if fp, ok := rs["fingerprint"].(string); ok {
				p["client-fingerprint"] = fp
			}
		}
		p["reality-opts"] = opts
	} else if securityMode == "tls" {
		if ts, ok := stream["tlsSettings"].(map[string]any); ok {
			if sni, ok := ts["serverName"].(string); ok && sni != "" {
				p["servername"] = sni
			}
			if fp, ok := ts["fingerprint"].(string); ok && fp != "" {
				p["client-fingerprint"] = fp
			}
			if alpn, ok := ts["alpn"].([]any); ok && len(alpn) > 0 {
				parts := make([]string, 0, len(alpn))
				for _, a := range alpn {
					parts = append(parts, fmt.Sprint(a))
				}
				p["alpn"] = parts
			}
			if ai, ok := ts["allowInsecure"].(bool); ok && ai {
				p["skip-cert-verify"] = true
			}
		}
	}
	switch network {
	case "ws":
		opts := map[string]any{}
		if ws, ok := stream["wsSettings"].(map[string]any); ok {
			if path, ok := ws["path"].(string); ok {
				opts["path"] = path
			}
			if h, ok := ws["headers"].(map[string]any); ok {
				if host, ok := h["Host"].(string); ok && host != "" {
					opts["headers"] = map[string]any{"Host": host}
				}
			}
		}
		p["ws-opts"] = opts
	case "grpc":
		opts := map[string]any{}
		if gs, ok := stream["grpcSettings"].(map[string]any); ok {
			if sn, ok := gs["serviceName"].(string); ok {
				opts["grpc-service-name"] = sn
			}
		}
		p["grpc-opts"] = opts
	case "h2", "http":
		opts := map[string]any{}
		if hs, ok := stream["httpSettings"].(map[string]any); ok {
			if path, ok := hs["path"].(string); ok {
				opts["path"] = []string{path}
			}
			if host, ok := hs["host"].([]any); ok {
				parts := make([]string, 0, len(host))
				for _, h := range host {
					parts = append(parts, fmt.Sprint(h))
				}
				opts["host"] = parts
			}
		}
		p["h2-opts"] = opts
	}
}

// jsonOutbound builds an outbound-like object from inbound+client (DB), not by parsing the URI.
func jsonOutbound(in *model.Inbound, cl model.Client, host, name, link string) map[string]any {
	if in.Protocol == "extra" {
		return map[string]any{
			"protocol": "extra",
			"tag":      name,
			"link":     link,
			"remark":   in.Remark,
		}
	}
	stream := protocol.ParseStream(in.StreamSettings)
	ob := map[string]any{
		"protocol":       string(in.Protocol),
		"tag":            name,
		"address":        host,
		"port":           in.Port,
		"email":          cl.Email,
		"remark":         in.Remark,
		"streamSettings": stream,
		"link":           link,
	}
	switch in.Protocol {
	case model.ProtoVLESS, model.ProtoVMess:
		ob["id"] = cl.UUID
		if cl.Flow != "" {
			ob["flow"] = cl.Flow
		}
	case model.ProtoTrojan:
		pw := cl.Password
		if pw == "" {
			pw = cl.UUID
		}
		ob["password"] = pw
		ob["id"] = cl.UUID
	case model.ProtoShadowsocks:
		settings := protocol.ParseSettings(in.Settings)
		method, _ := settings["method"].(string)
		pw := cl.Password
		if pw == "" {
			pw = cl.UUID
		}
		ob["method"] = method
		ob["password"] = pw
		ob["id"] = cl.UUID
	default:
		ob["id"] = cl.UUID
		if cl.Password != "" {
			ob["password"] = cl.Password
		}
		if cl.Flow != "" {
			ob["flow"] = cl.Flow
		}
	}
	return ob
}

func loadEnabledSubBalancers() []model.SubBalancer {
	var rows []model.SubBalancer
	_ = database.DB.Where("enable = ?", true).Order("id asc").Find(&rows)
	return rows
}

func subBalancerMatches(b model.SubBalancer, entries []subEntry) bool {
	sel := strings.TrimSpace(b.Selector)
	if sel == "" {
		return true // empty selector = all proxies in this sub
	}
	parts := strings.Split(sel, ",")
	emails := map[string]bool{}
	tags := map[string]bool{}
	for _, e := range entries {
		if e.Client.Email != "" {
			emails[strings.ToLower(e.Client.Email)] = true
		}
		if e.Inbound.Tag != "" {
			tags[e.Inbound.Tag] = true
		}
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if emails[strings.ToLower(p)] || tags[p] {
			return true
		}
	}
	return false
}

func clashStrategyType(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "fallback":
		return "fallback"
	case "round-robin", "roundrobin":
		return "load-balance"
	default:
		return "url-test"
	}
}

func clashBalancerGroups(entries []subEntry, names []string) []map[string]any {
	if len(names) == 0 {
		return nil
	}
	out := []map[string]any{}
	for _, b := range loadEnabledSubBalancers() {
		if !subBalancerMatches(b, entries) {
			continue
		}
		g := map[string]any{
			"name":    b.Name,
			"type":    clashStrategyType(b.Strategy),
			"proxies": names,
		}
		if g["type"] == "url-test" {
			g["url"] = "http://www.gstatic.com/generate_204"
			g["interval"] = 300
		}
		out = append(out, g)
	}
	return out
}

func subBalancerGroups(entries []subEntry, names []string) []map[string]any {
	if len(names) == 0 {
		return nil
	}
	out := []map[string]any{}
	for _, b := range loadEnabledSubBalancers() {
		if !subBalancerMatches(b, entries) {
			continue
		}
		out = append(out, map[string]any{
			"protocol": "balancer",
			"tag":      b.Name,
			"strategy": b.Strategy,
			"selector": names,
			"remark":   b.Remark,
		})
	}
	return out
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
			if settings, ok := rs["settings"].(map[string]any); ok {
				if sni, _ := settings["serverName"].(string); sni != "" {
					tls["server_name"] = sni
				}
				if fp, _ := settings["fingerprint"].(string); fp != "" {
					tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
				}
			}
			if pbk := protocol.ResolveRealityPublicKey(rs); pbk != "" {
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
	case "subJsonEnable", "subClashEnable", "subForceTLS", "tgBotEnable", "tgNotifyLogin", "tgNotifyTraffic", "twoFactorEnable",
		"emailEnable", "emailNotifyLogin", "emailNotifyTraffic", "discordEnable", "discordNotifyLogin", "discordNotifyTraffic":
		v := strings.ToLower(strings.TrimSpace(value))
		switch v {
		case "true", "false", "1", "0", "yes", "no", "on", "off", "":
		default:
			return fmt.Errorf("invalid %s", key)
		}
	case "subURI":
		v := strings.TrimSpace(value)
		if v == "" {
			return nil
		}
		if len(v) > 512 || strings.ContainsAny(v, " \t\r\n") {
			return fmt.Errorf("invalid subURI")
		}
		if !strings.Contains(v, "://") {
			v = "https://" + v
		}
		u, err := url.Parse(v)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("invalid subURI")
		}
	case "smtpPort":
		v := strings.TrimSpace(value)
		if v == "" {
			return nil
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("invalid smtpPort")
		}
	case "smtpHost":
		v := strings.TrimSpace(value)
		if v == "" {
			return nil
		}
		if len(v) > 255 || strings.ContainsAny(v, " \t\r\n/:@") {
			return fmt.Errorf("invalid smtpHost")
		}
	case "smtpFrom", "smtpUser":
		if len(value) > 256 {
			return fmt.Errorf("%s too long", key)
		}
	case "smtpPass":
		if value == "***" || strings.Contains(value, "…") {
			return nil
		}
		if len(value) > 256 {
			return fmt.Errorf("smtpPass too long")
		}
	case "discordWebhook":
		v := strings.TrimSpace(value)
		if v == "" || v == "***" || strings.Contains(v, "…") {
			return nil
		}
		if len(v) > 512 {
			return fmt.Errorf("discordWebhook too long")
		}
		if !strings.HasPrefix(v, "https://discord.com/api/webhooks/") &&
			!strings.HasPrefix(v, "https://discordapp.com/api/webhooks/") {
			return fmt.Errorf("discordWebhook must be https://discord.com/api/webhooks/…")
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
