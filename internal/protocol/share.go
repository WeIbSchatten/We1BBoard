package protocol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/we1bboard/we1bboard/internal/database/model"
)

// formatShareHost brackets IPv6 literals for URI authority (RFC 3986).
func formatShareHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		return "[" + host + "]"
	}
	return host
}

func joinShareAddr(host string, port int) string {
	return net.JoinHostPort(strings.Trim(formatShareHost(host), "[]"), strconv.Itoa(port))
}

func fragmentEscape(s string) string {
	// Clients expect percent-encoding in the fragment (spaces as %20, not +).
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case json.Number:
		return t.String()
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
}

func firstString(list any) string {
	switch t := list.(type) {
	case []any:
		if len(t) > 0 {
			return asString(t[0])
		}
	case []string:
		if len(t) > 0 {
			return t[0]
		}
	case string:
		return t
	}
	return ""
}

func mapStringCI(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, want := range keys {
		for k, v := range m {
			if strings.EqualFold(k, want) {
				if s := asString(v); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func headerHost(headers any) string {
	switch h := headers.(type) {
	case map[string]any:
		return mapStringCI(h, "Host", "host")
	case map[string]string:
		for k, v := range h {
			if strings.EqualFold(k, "Host") && v != "" {
				return v
			}
		}
	case []any:
		// rare: [{Host: ["a.com"]}]
		for _, item := range h {
			if m, ok := item.(map[string]any); ok {
				if v := mapStringCI(m, "Host", "host"); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

func remarkFor(in *model.Inbound, c model.Client) string {
	if c.Email != "" {
		return c.Email
	}
	if in.Remark != "" {
		return in.Remark
	}
	return string(in.Protocol)
}

// ShareLink builds a client-importable URI (3x-ui compatible).
func (a *xrayProxy) ShareLink(in *model.Inbound, c model.Client, host string) (string, error) {
	stream := ParseStream(in.StreamSettings)
	network := asString(stream["network"])
	security := asString(stream["security"])
	if network == "" {
		network = "tcp"
	}
	if security == "" {
		security = "none"
	}
	port := in.Port
	if port < 1 {
		port = 443
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}

	switch a.proto {
	case model.ProtoVLESS:
		return vlessLink(in, c, host, port, network, security, stream), nil
	case model.ProtoVMess:
		return vmessLink(in, c, host, port, network, security, stream), nil
	case model.ProtoTrojan:
		return trojanLink(in, c, host, port, network, security, stream), nil
	case model.ProtoShadowsocks:
		return ssLink(in, c, host, port), nil
	default:
		return fmt.Sprintf("%s://%s@%s#%s", a.proto, c.UUID, joinShareAddr(host, port), fragmentEscape(remarkFor(in, c))), nil
	}
}

func vlessLink(in *model.Inbound, c model.Client, host string, port int, network, security string, stream map[string]any) string {
	q := url.Values{}
	encryption := "none"
	if settings := ParseSettings(in.Settings); settings != nil {
		if enc := asString(settings["encryption"]); enc != "" {
			encryption = enc
		}
	}
	q.Set("encryption", encryption)
	q.Set("type", network)
	q.Set("security", security)
	applyNetworkQuery(q, stream, network)
	applySecurityQuery(q, stream, security)
	if c.Flow != "" && (security == "tls" || security == "reality") && (network == "tcp" || network == "xhttp") {
		q.Set("flow", c.Flow)
	}
	return fmt.Sprintf("vless://%s@%s?%s#%s",
		c.UUID, joinShareAddr(host, port), q.Encode(), fragmentEscape(remarkFor(in, c)))
}

func trojanLink(in *model.Inbound, c model.Client, host string, port int, network, security string, stream map[string]any) string {
	pw := c.Password
	if pw == "" {
		pw = c.UUID
	}
	q := url.Values{}
	q.Set("type", network)
	q.Set("security", security)
	applyNetworkQuery(q, stream, network)
	applySecurityQuery(q, stream, security)
	return fmt.Sprintf("trojan://%s@%s?%s#%s",
		url.PathEscape(pw), joinShareAddr(host, port), q.Encode(), fragmentEscape(remarkFor(in, c)))
}

func ssLink(in *model.Inbound, c model.Client, host string, port int) string {
	settings := ParseSettings(in.Settings)
	method := asString(settings["method"])
	if method == "" {
		method = "aes-256-gcm"
	}
	pw := c.Password
	if pw == "" {
		pw = c.UUID
	}
	// SIP002: method:password — SS2022 uses standard userinfo percent-encoding in some clients;
	// base64url userinfo remains the widely compatible form (3x-ui / v2rayN).
	userinfo := base64.RawURLEncoding.EncodeToString([]byte(method + ":" + pw))
	return fmt.Sprintf("ss://%s@%s#%s", userinfo, joinShareAddr(host, port), fragmentEscape(remarkFor(in, c)))
}

func vmessLink(in *model.Inbound, c model.Client, host string, port int, network, security string, stream map[string]any) string {
	obj := map[string]any{
		"v":   "2",
		"ps":  remarkFor(in, c),
		"add": host,
		"port": strconv.Itoa(port),
		"id":  c.UUID,
		"aid": "0",
		"scy": "auto",
		"net": network,
		"type": "none",
		"host": "",
		"path": "",
		"tls": "",
		"sni": "",
		"alpn": "",
		"fp": "",
	}
	if security == "tls" || security == "reality" {
		obj["tls"] = security
	}
	applyVmessNetwork(obj, stream, network)
	applyVmessSecurity(obj, stream, security)
	b, _ := json.Marshal(obj)
	return "vmess://" + base64.StdEncoding.EncodeToString(b)
}

func applyNetworkQuery(q url.Values, stream map[string]any, network string) {
	switch network {
	case "tcp":
		if tcp, ok := stream["tcpSettings"].(map[string]any); ok {
			if header, ok := tcp["header"].(map[string]any); ok {
				ht := asString(header["type"])
				if ht == "http" {
					q.Set("headerType", "http")
					if req, ok := header["request"].(map[string]any); ok {
						if path := firstString(req["path"]); path != "" {
							q.Set("path", path)
						}
						if h := headerHost(req["headers"]); h != "" {
							q.Set("host", h)
						}
					}
					if resp, ok := header["response"].(map[string]any); ok {
						if h := headerHost(resp["headers"]); h != "" && q.Get("host") == "" {
							q.Set("host", h)
						}
					}
				}
			}
		}
	case "kcp", "mkcp":
		if ks, ok := stream["kcpSettings"].(map[string]any); ok {
			if seed := asString(ks["seed"]); seed != "" {
				q.Set("seed", seed)
			}
			if header, ok := ks["header"].(map[string]any); ok {
				if ht := asString(header["type"]); ht != "" && ht != "none" {
					q.Set("headerType", ht)
				}
			}
			if mtu := asString(ks["mtu"]); mtu != "" {
				q.Set("mtu", mtu)
			}
			if tti := asString(ks["tti"]); tti != "" {
				q.Set("tti", tti)
			}
		}
	case "ws", "websocket":
		if ws, ok := stream["wsSettings"].(map[string]any); ok {
			if p := asString(ws["path"]); p != "" {
				q.Set("path", p)
			}
			host := asString(ws["host"])
			if host == "" {
				host = headerHost(ws["headers"])
			}
			if host != "" {
				q.Set("host", host)
			}
		}
	case "grpc", "gun":
		if gs, ok := stream["grpcSettings"].(map[string]any); ok {
			if sn := asString(gs["serviceName"]); sn != "" {
				q.Set("serviceName", sn)
			}
			if auth := asString(gs["authority"]); auth != "" {
				q.Set("authority", auth)
			}
			if multi, ok := gs["multiMode"].(bool); ok && multi {
				q.Set("mode", "multi")
			}
		}
	case "httpupgrade":
		if hs, ok := stream["httpupgradeSettings"].(map[string]any); ok {
			if p := asString(hs["path"]); p != "" {
				q.Set("path", p)
			}
			host := asString(hs["host"])
			if host == "" {
				host = headerHost(hs["headers"])
			}
			if host != "" {
				q.Set("host", host)
			}
		}
	case "xhttp", "splithttp":
		if xs, ok := stream["xhttpSettings"].(map[string]any); ok {
			if p := asString(xs["path"]); p != "" {
				q.Set("path", p)
			}
			host := asString(xs["host"])
			if host == "" {
				host = headerHost(xs["headers"])
			}
			if host != "" {
				q.Set("host", host)
			}
			if mode := asString(xs["mode"]); mode != "" {
				q.Set("mode", mode)
			}
		}
	case "h2", "http":
		if hs, ok := stream["httpSettings"].(map[string]any); ok {
			if path := firstString(hs["path"]); path != "" {
				q.Set("path", path)
			}
			if host := firstString(hs["host"]); host != "" {
				q.Set("host", host)
			}
		}
	}
}

func applySecurityQuery(q url.Values, stream map[string]any, security string) {
	switch security {
	case "reality":
		rs, _ := stream["realitySettings"].(map[string]any)
		if rs == nil {
			return
		}
		settings, _ := rs["settings"].(map[string]any)
		pbk := ResolveRealityPublicKey(rs)
		if pbk != "" {
			q.Set("pbk", pbk)
		}
		fp := asString(rs["fingerprint"])
		if fp == "" && settings != nil {
			fp = asString(settings["fingerprint"])
		}
		if fp == "" {
			fp = "chrome"
		}
		q.Set("fp", fp)

		sni := ""
		if settings != nil {
			sni = asString(settings["serverName"])
		}
		if sni == "" {
			sni = firstString(rs["serverNames"])
		}
		if sni == "" {
			dest := asString(rs["dest"])
			if dest == "" {
				dest = asString(rs["target"])
			}
			if dest != "" {
				if h, _, err := net.SplitHostPort(dest); err == nil {
					sni = h
				} else {
					sni = strings.Split(dest, ":")[0]
				}
			}
		}
		if sni != "" {
			q.Set("sni", sni)
		}

		if sid := firstString(rs["shortIds"]); sid != "" {
			q.Set("sid", sid)
		}
		spx := asString(rs["spiderX"])
		if spx == "" && settings != nil {
			spx = asString(settings["spiderX"])
		}
		if spx == "" {
			spx = "/"
		}
		q.Set("spx", spx)

	case "tls":
		ts, _ := stream["tlsSettings"].(map[string]any)
		if ts == nil {
			return
		}
		settings, _ := ts["settings"].(map[string]any)
		if sni := asString(ts["serverName"]); sni != "" {
			q.Set("sni", sni)
		}
		fp := asString(ts["fingerprint"])
		if fp == "" && settings != nil {
			fp = asString(settings["fingerprint"])
		}
		if fp != "" {
			q.Set("fp", fp)
		}
		if alpn, ok := ts["alpn"].([]any); ok && len(alpn) > 0 {
			parts := make([]string, 0, len(alpn))
			for _, a := range alpn {
				if s := asString(a); s != "" {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				q.Set("alpn", strings.Join(parts, ","))
			}
		}
		allowInsecure := false
		if v, ok := ts["allowInsecure"].(bool); ok {
			allowInsecure = v
		}
		if settings != nil {
			if v, ok := settings["allowInsecure"].(bool); ok {
				allowInsecure = v
			}
		}
		if allowInsecure {
			q.Set("allowInsecure", "1")
		}
	}
}

func applyVmessNetwork(obj map[string]any, stream map[string]any, network string) {
	switch network {
	case "tcp":
		if tcp, ok := stream["tcpSettings"].(map[string]any); ok {
			if header, ok := tcp["header"].(map[string]any); ok {
				if asString(header["type"]) == "http" {
					obj["type"] = "http"
					if req, ok := header["request"].(map[string]any); ok {
						if path := firstString(req["path"]); path != "" {
							obj["path"] = path
						}
						if h := headerHost(req["headers"]); h != "" {
							obj["host"] = h
						}
					}
				}
			}
		}
	case "ws", "websocket":
		if ws, ok := stream["wsSettings"].(map[string]any); ok {
			if p := asString(ws["path"]); p != "" {
				obj["path"] = p
			}
			host := asString(ws["host"])
			if host == "" {
				host = headerHost(ws["headers"])
			}
			if host != "" {
				obj["host"] = host
			}
		}
	case "grpc", "gun":
		if gs, ok := stream["grpcSettings"].(map[string]any); ok {
			if sn := asString(gs["serviceName"]); sn != "" {
				obj["path"] = sn
			}
			if multi, ok := gs["multiMode"].(bool); ok && multi {
				obj["type"] = "multi"
			}
		}
	case "httpupgrade":
		if hs, ok := stream["httpupgradeSettings"].(map[string]any); ok {
			if p := asString(hs["path"]); p != "" {
				obj["path"] = p
			}
			host := asString(hs["host"])
			if host == "" {
				host = headerHost(hs["headers"])
			}
			if host != "" {
				obj["host"] = host
			}
		}
	case "xhttp", "splithttp":
		obj["net"] = "xhttp"
		if xs, ok := stream["xhttpSettings"].(map[string]any); ok {
			if p := asString(xs["path"]); p != "" {
				obj["path"] = p
			}
			host := asString(xs["host"])
			if host == "" {
				host = headerHost(xs["headers"])
			}
			if host != "" {
				obj["host"] = host
			}
			if mode := asString(xs["mode"]); mode != "" {
				obj["type"] = mode
			}
		}
	case "h2", "http":
		obj["net"] = "h2"
		if hs, ok := stream["httpSettings"].(map[string]any); ok {
			if path := firstString(hs["path"]); path != "" {
				obj["path"] = path
			}
			if host := firstString(hs["host"]); host != "" {
				obj["host"] = host
			}
		}
	case "kcp", "mkcp":
		obj["net"] = "kcp"
		if ks, ok := stream["kcpSettings"].(map[string]any); ok {
			if seed := asString(ks["seed"]); seed != "" {
				obj["path"] = seed
			}
			if header, ok := ks["header"].(map[string]any); ok {
				if ht := asString(header["type"]); ht != "" {
					obj["type"] = ht
				}
			}
		}
	}
}

func applyVmessSecurity(obj map[string]any, stream map[string]any, security string) {
	switch security {
	case "tls":
		ts, _ := stream["tlsSettings"].(map[string]any)
		if ts == nil {
			return
		}
		settings, _ := ts["settings"].(map[string]any)
		if sni := asString(ts["serverName"]); sni != "" {
			obj["sni"] = sni
		}
		fp := asString(ts["fingerprint"])
		if fp == "" && settings != nil {
			fp = asString(settings["fingerprint"])
		}
		if fp != "" {
			obj["fp"] = fp
		}
		if alpn, ok := ts["alpn"].([]any); ok && len(alpn) > 0 {
			parts := make([]string, 0, len(alpn))
			for _, a := range alpn {
				if s := asString(a); s != "" {
					parts = append(parts, s)
				}
			}
			obj["alpn"] = strings.Join(parts, ",")
		}
	case "reality":
		// VMess+REALITY is rare; still emit tls=reality + sni/fp for clients that accept it.
		rs, _ := stream["realitySettings"].(map[string]any)
		if rs == nil {
			return
		}
		settings, _ := rs["settings"].(map[string]any)
		sni := ""
		if settings != nil {
			sni = asString(settings["serverName"])
		}
		if sni == "" {
			sni = firstString(rs["serverNames"])
		}
		if sni != "" {
			obj["sni"] = sni
		}
		fp := asString(rs["fingerprint"])
		if fp == "" && settings != nil {
			fp = asString(settings["fingerprint"])
		}
		if fp != "" {
			obj["fp"] = fp
		}
	}
}
