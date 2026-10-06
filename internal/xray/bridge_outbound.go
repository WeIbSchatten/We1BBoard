package xray

import (
	"encoding/json"

	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/protocol"
)

// buildBridgeOutbound builds an Xray outbound for RU→EU (or any) chaining.
// Supports any Xray outbound protocol: vless, vmess, trojan, shadowsocks, socks, http, wireguard, …
// If DialerSettings / DialerStreamSettings are set, they are used as the source of truth (3x-ui style).
func buildBridgeOutbound(b model.Bridge, tag string) map[string]any {
	proto := b.DialerProtocol
	if proto == "" {
		proto = "vless"
	}

	var settings map[string]any
	if b.DialerSettings != "" && b.DialerSettings != "{}" {
		settings = protocol.ParseSettings(b.DialerSettings)
	} else {
		settings = defaultBridgeSettings(b, proto)
	}

	var stream map[string]any
	if b.DialerStreamSettings != "" && b.DialerStreamSettings != "{}" {
		stream = protocol.ParseStream(b.DialerStreamSettings)
	} else {
		stream = defaultBridgeStream(b)
	}

	out := map[string]any{
		"tag":      tag,
		"protocol": proto,
		"settings": settings,
	}
	// freedom/blackhole/dns/wireguard may omit or use different stream
	switch proto {
	case "freedom", "blackhole", "dns", "wireguard":
		if len(stream) > 0 && proto == "wireguard" {
			// wireguard usually has no streamSettings
		}
	default:
		out["streamSettings"] = stream
	}
	return out
}

func defaultBridgeSettings(b model.Bridge, proto string) map[string]any {
	addr := b.DialerAddress
	port := b.DialerPort
	if port == 0 {
		port = 443
	}
	uuid := b.DialerUUID
	pw := b.DialerPassword
	if pw == "" {
		pw = uuid
	}
	email := b.DialerEmail
	if email == "" {
		email = "bridge@we1b"
	}

	switch proto {
	case "vless":
		user := map[string]any{"id": uuid, "encryption": "none", "email": email}
		if b.DialerFlow != "" {
			user["flow"] = b.DialerFlow
		}
		return map[string]any{
			"vnext": []map[string]any{{
				"address": addr,
				"port":    port,
				"users":   []map[string]any{user},
			}},
		}
	case "vmess":
		return map[string]any{
			"vnext": []map[string]any{{
				"address": addr,
				"port":    port,
				"users": []map[string]any{{
					"id": uuid, "alterId": 0, "security": "auto", "email": email,
				}},
			}},
		}
	case "trojan":
		return map[string]any{
			"servers": []map[string]any{{
				"address": addr, "port": port, "password": pw, "email": email,
			}},
		}
	case "shadowsocks":
		method := b.DialerMethod
		if method == "" {
			method = "aes-256-gcm"
		}
		return map[string]any{
			"servers": []map[string]any{{
				"address": addr, "port": port, "method": method, "password": pw, "email": email,
			}},
		}
	case "socks":
		server := map[string]any{"address": addr, "port": port}
		if b.DialerEmail != "" || pw != "" {
			server["users"] = []map[string]any{{"user": email, "pass": pw}}
		}
		return map[string]any{"servers": []map[string]any{server}}
	case "http":
		server := map[string]any{"address": addr, "port": port}
		if b.DialerEmail != "" || pw != "" {
			server["users"] = []map[string]any{{"user": email, "pass": pw}}
		}
		return map[string]any{"servers": []map[string]any{server}}
	case "wireguard":
		// Expect full JSON in DialerSettings for peers/secretKey; minimal stub:
		s := protocol.ParseSettings(b.DialerSettings)
		if len(s) == 0 {
			return map[string]any{
				"secretKey": b.DialerPassword,
				"address":   []string{"10.0.0.2/32"},
				"peers": []map[string]any{{
					"endpoint":   addr + ":" + itoa(port),
					"publicKey":  b.DialerPublicKey,
					"allowedIPs": []string{"0.0.0.0/0", "::/0"},
				}},
			}
		}
		return s
	default:
		// Unknown protocol: pass through address hint in settings for operator-edited JSON
		raw := protocol.ParseSettings(b.DialerSettings)
		if len(raw) == 0 {
			return map[string]any{
				"address": addr,
				"port":    port,
				"note":    "set dialerSettings JSON for protocol " + proto,
			}
		}
		return raw
	}
}

func defaultBridgeStream(b model.Bridge) map[string]any {
	network := b.DialerNetwork
	if network == "" {
		network = "tcp"
	}
	security := b.DialerSecurity
	if security == "" {
		security = "none"
	}
	stream := map[string]any{
		"network":  network,
		"security": security,
	}

	switch network {
	case "ws":
		ws := map[string]any{}
		if b.DialerPath != "" {
			ws["path"] = b.DialerPath
		}
		if b.DialerHost != "" {
			ws["headers"] = map[string]any{"Host": b.DialerHost}
		}
		stream["wsSettings"] = ws
	case "grpc":
		gs := map[string]any{}
		if b.DialerServiceName != "" {
			gs["serviceName"] = b.DialerServiceName
		}
		stream["grpcSettings"] = gs
	case "httpupgrade":
		stream["httpupgradeSettings"] = map[string]any{"path": nullable(b.DialerPath, "/")}
	case "xhttp":
		stream["xhttpSettings"] = map[string]any{"path": nullable(b.DialerPath, "/"), "mode": "auto"}
	case "kcp":
		stream["kcpSettings"] = map[string]any{"mtu": 1350}
	}

	fp := b.DialerFingerprt
	if fp == "" {
		fp = "chrome"
	}
	switch security {
	case "reality":
		stream["realitySettings"] = map[string]any{
			"serverName":  b.DialerSNI,
			"fingerprint": fp,
			"publicKey":   b.DialerPublicKey,
			"shortId":     b.DialerShortID,
			"spiderX":     "/",
		}
	case "tls", "xtls":
		tls := map[string]any{"fingerprint": fp}
		if b.DialerSNI != "" {
			tls["serverName"] = b.DialerSNI
		}
		stream["tlsSettings"] = tls
	}
	return stream
}

func nullable(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
