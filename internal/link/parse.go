// Package link parses share-link URLs (vless/vmess/trojan/ss) into Xray outbound shapes.
package link

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Parsed is a minimal outbound derived from a share link.
type Parsed struct {
	Protocol       string
	Tag            string
	Remark         string
	Settings       string // JSON
	StreamSettings string // JSON
}

// ParseOutboundLink dispatches to protocol-specific parsers.
func ParseOutboundLink(raw string) (*Parsed, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("empty link")
	}
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "vmess://"):
		return parseVmess(s)
	case strings.HasPrefix(lower, "vless://"):
		return parseVless(s)
	case strings.HasPrefix(lower, "trojan://"):
		return parseTrojan(s)
	case strings.HasPrefix(lower, "ss://"):
		return parseShadowsocks(s)
	default:
		return nil, fmt.Errorf("unsupported link scheme")
	}
}

// ExtractShareLinks splits subscription body into share-link lines (base64-aware).
func ExtractShareLinks(body string) []string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return nil
	}
	text := trimmed
	if looksLikeBase64(trimmed) {
		if dec, err := decodeBase64(trimmed); err == nil && dec != "" {
			text = dec
		}
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		low := strings.ToLower(line)
		if strings.HasPrefix(low, "vmess://") ||
			strings.HasPrefix(low, "vless://") ||
			strings.HasPrefix(low, "trojan://") ||
			strings.HasPrefix(low, "ss://") ||
			strings.HasPrefix(low, "hysteria2://") ||
			strings.HasPrefix(low, "hy2://") ||
			strings.HasPrefix(low, "wireguard://") ||
			strings.HasPrefix(low, "wg://") {
			out = append(out, line)
		}
	}
	return out
}

func looksLikeBase64(s string) bool {
	if strings.Contains(s, "://") {
		return false
	}
	if strings.ContainsAny(s, " \t\n\r") {
		return false
	}
	if len(s) < 16 {
		return false
	}
	for _, c := range s {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '+' || c == '/' || c == '=' || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func decodeBase64(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func parseVmess(link string) (*Parsed, error) {
	payload := strings.TrimPrefix(link, "vmess://")
	payload = strings.TrimPrefix(payload, "VMESS://")
	raw, err := decodeBase64(payload)
	if err != nil {
		return nil, fmt.Errorf("vmess decode: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("vmess json: %w", err)
	}
	addr, _ := m["add"].(string)
	id, _ := m["id"].(string)
	ps, _ := m["ps"].(string)
	netw, _ := m["net"].(string)
	if netw == "" {
		netw = "tcp"
	}
	tlsVal, _ := m["tls"].(string)
	security := "none"
	if tlsVal == "tls" {
		security = "tls"
	}
	port := anyToInt(m["port"], 443)
	scy, _ := m["scy"].(string)
	if scy == "" || scy == "none" || scy == "zero" {
		scy = "auto"
	}
	settings, _ := json.Marshal(map[string]any{
		"vnext": []map[string]any{{
			"address": addr,
			"port":    port,
			"users": []map[string]any{{
				"id":       id,
				"alterId":  0,
				"security": scy,
			}},
		}},
	})
	stream := buildStream(netw, security)
	if security == "tls" {
		tls := stream["tlsSettings"].(map[string]any)
		if sni, ok := m["sni"].(string); ok {
			tls["serverName"] = sni
		}
		if fp, ok := m["fp"].(string); ok {
			tls["fingerprint"] = fp
		}
	}
	applyVmessTransport(stream, m)
	streamJSON, _ := json.Marshal(stream)
	return &Parsed{
		Protocol:       "vmess",
		Tag:            ps,
		Remark:         ps,
		Settings:       string(settings),
		StreamSettings: string(streamJSON),
	}, nil
}

func applyVmessTransport(stream map[string]any, m map[string]any) {
	netw, _ := stream["network"].(string)
	host, _ := m["host"].(string)
	path, _ := m["path"].(string)
	switch netw {
	case "ws":
		ws := stream["wsSettings"].(map[string]any)
		ws["host"] = host
		if path != "" {
			ws["path"] = path
		}
	case "grpc":
		grpc := stream["grpcSettings"].(map[string]any)
		grpc["serviceName"] = path
	case "httpupgrade":
		hu := stream["httpupgradeSettings"].(map[string]any)
		hu["host"] = host
		if path != "" {
			hu["path"] = path
		}
	}
}

func parseVless(link string) (*Parsed, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	id := u.User.Username()
	host := u.Hostname()
	port := 443
	if u.Port() != "" {
		port, _ = strconv.Atoi(u.Port())
	}
	q := u.Query()
	network := q.Get("type")
	if network == "" {
		network = "tcp"
	}
	security := q.Get("security")
	if security == "" {
		security = "none"
	}
	flow := q.Get("flow")
	settings, _ := json.Marshal(map[string]any{
		"vnext": []map[string]any{{
			"address": host,
			"port":    port,
			"users": []map[string]any{{
				"id":         id,
				"encryption": "none",
				"flow":       flow,
			}},
		}},
	})
	stream := buildStream(network, security)
	applyURLTransport(stream, q)
	applyURLSecurity(stream, q)
	streamJSON, _ := json.Marshal(stream)
	remark := decodeFragment(u)
	return &Parsed{
		Protocol:       "vless",
		Tag:            remark,
		Remark:         remark,
		Settings:       string(settings),
		StreamSettings: string(streamJSON),
	}, nil
}

func parseTrojan(link string) (*Parsed, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	password, _ := u.User.Password()
	if password == "" {
		password = u.User.Username()
	}
	host := u.Hostname()
	port := 443
	if u.Port() != "" {
		port, _ = strconv.Atoi(u.Port())
	}
	q := u.Query()
	network := q.Get("type")
	if network == "" {
		network = "tcp"
	}
	security := q.Get("security")
	if security == "" {
		security = "tls"
	}
	settings, _ := json.Marshal(map[string]any{
		"servers": []map[string]any{{
			"address":  host,
			"port":     port,
			"password": password,
		}},
	})
	stream := buildStream(network, security)
	applyURLTransport(stream, q)
	applyURLSecurity(stream, q)
	streamJSON, _ := json.Marshal(stream)
	remark := decodeFragment(u)
	return &Parsed{
		Protocol:       "trojan",
		Tag:            remark,
		Remark:         remark,
		Settings:       string(settings),
		StreamSettings: string(streamJSON),
	}, nil
}

func parseShadowsocks(link string) (*Parsed, error) {
	hashIdx := strings.Index(link, "#")
	remark := ""
	core := link
	if hashIdx >= 0 {
		remark, _ = url.QueryUnescape(link[hashIdx+1:])
		core = link[:hashIdx]
	}
	queryIdx := strings.Index(core, "?")
	rawQuery := ""
	if queryIdx >= 0 {
		rawQuery = core[queryIdx+1:]
		core = core[:queryIdx]
	}
	var method, password, host string
	var port int
	at := strings.Index(core, "@")
	if at >= 0 {
		userInfo := core[len("ss://"):at]
		if strings.Contains(userInfo, ":") {
			userInfo, _ = url.QueryUnescape(userInfo)
		} else {
			if dec, err := decodeBase64(userInfo); err == nil {
				userInfo = dec
			}
		}
		hostPort := strings.TrimRight(core[at+1:], "/")
		colon := strings.LastIndex(hostPort, ":")
		if colon < 0 {
			return nil, fmt.Errorf("ss: missing port")
		}
		host = hostPort[:colon]
		port, _ = strconv.Atoi(hostPort[colon+1:])
		sep := strings.Index(userInfo, ":")
		if sep < 0 {
			return nil, fmt.Errorf("ss: bad userinfo")
		}
		method = userInfo[:sep]
		password = userInfo[sep+1:]
	} else {
		decoded, err := decodeBase64(strings.TrimPrefix(core, "ss://"))
		if err != nil {
			return nil, err
		}
		at2 := strings.Index(decoded, "@")
		if at2 < 0 {
			return nil, fmt.Errorf("ss: legacy decode failed")
		}
		userInfo := decoded[:at2]
		hostPort := decoded[at2+1:]
		colon := strings.LastIndex(hostPort, ":")
		if colon < 0 {
			return nil, fmt.Errorf("ss: missing port")
		}
		host = hostPort[:colon]
		port, _ = strconv.Atoi(hostPort[colon+1:])
		sep := strings.Index(userInfo, ":")
		if sep < 0 {
			return nil, fmt.Errorf("ss: bad userinfo")
		}
		method = userInfo[:sep]
		password = userInfo[sep+1:]
	}
	if port == 0 {
		port = 443
	}
	q, _ := url.ParseQuery(rawQuery)
	network := q.Get("type")
	if network == "" {
		network = "tcp"
	}
	security := q.Get("security")
	if security == "" {
		security = "none"
	}
	settings, _ := json.Marshal(map[string]any{
		"servers": []map[string]any{{
			"address":  host,
			"port":     port,
			"method":   method,
			"password": password,
		}},
	})
	stream := buildStream(network, security)
	applyURLTransport(stream, q)
	applyURLSecurity(stream, q)
	streamJSON, _ := json.Marshal(stream)
	return &Parsed{
		Protocol:       "shadowsocks",
		Tag:            remark,
		Remark:         remark,
		Settings:       string(settings),
		StreamSettings: string(streamJSON),
	}, nil
}

func buildStream(network, security string) map[string]any {
	stream := map[string]any{
		"network":  network,
		"security": security,
	}
	switch network {
	case "ws":
		stream["wsSettings"] = map[string]any{"path": "/", "host": "", "headers": map[string]any{}}
	case "grpc":
		stream["grpcSettings"] = map[string]any{"serviceName": "", "authority": "", "multiMode": false}
	case "httpupgrade":
		stream["httpupgradeSettings"] = map[string]any{"path": "/", "host": "", "headers": map[string]any{}}
	case "xhttp":
		stream["xhttpSettings"] = map[string]any{"path": "/", "host": "", "mode": "auto", "headers": map[string]any{}}
	default:
		stream["tcpSettings"] = map[string]any{"header": map[string]any{"type": "none"}}
	}
	if security == "tls" {
		stream["tlsSettings"] = map[string]any{
			"serverName": "", "alpn": []string{}, "fingerprint": "",
		}
	} else if security == "reality" {
		stream["realitySettings"] = map[string]any{
			"publicKey": "", "fingerprint": "chrome", "serverName": "", "shortId": "", "spiderX": "",
		}
	}
	return stream
}

func applyURLTransport(stream map[string]any, q url.Values) {
	network, _ := stream["network"].(string)
	host := q.Get("host")
	path := q.Get("path")
	if path == "" {
		path = "/"
	}
	switch network {
	case "ws":
		ws := stream["wsSettings"].(map[string]any)
		ws["host"] = host
		ws["path"] = path
	case "grpc":
		grpc := stream["grpcSettings"].(map[string]any)
		sn := q.Get("serviceName")
		if sn == "" {
			sn = q.Get("path")
		}
		grpc["serviceName"] = sn
		grpc["authority"] = q.Get("authority")
		grpc["multiMode"] = q.Get("mode") == "multi"
	case "httpupgrade":
		hu := stream["httpupgradeSettings"].(map[string]any)
		hu["host"] = host
		hu["path"] = path
	case "xhttp":
		xh := stream["xhttpSettings"].(map[string]any)
		xh["host"] = host
		xh["path"] = path
		if m := q.Get("mode"); m != "" {
			xh["mode"] = m
		}
	}
}

func applyURLSecurity(stream map[string]any, q url.Values) {
	sec, _ := stream["security"].(string)
	switch sec {
	case "tls":
		tls := stream["tlsSettings"].(map[string]any)
		tls["serverName"] = q.Get("sni")
		tls["fingerprint"] = q.Get("fp")
		if alpn := q.Get("alpn"); alpn != "" {
			tls["alpn"] = strings.Split(alpn, ",")
		}
	case "reality":
		reality := stream["realitySettings"].(map[string]any)
		reality["serverName"] = q.Get("sni")
		fp := q.Get("fp")
		if fp == "" {
			fp = "chrome"
		}
		reality["fingerprint"] = fp
		reality["publicKey"] = q.Get("pbk")
		reality["shortId"] = q.Get("sid")
		reality["spiderX"] = q.Get("spx")
	}
}

func decodeFragment(u *url.URL) string {
	if u.Fragment == "" {
		return ""
	}
	s, err := url.QueryUnescape(u.Fragment)
	if err != nil {
		return u.Fragment
	}
	return s
}

func anyToInt(v any, def int) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n, err := strconv.Atoi(t)
		if err == nil {
			return n
		}
	}
	return def
}
