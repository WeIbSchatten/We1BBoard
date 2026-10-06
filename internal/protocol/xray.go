package protocol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/we1bboard/we1bboard/internal/database/model"
)

type xrayProxy struct {
	proto model.Protocol
}

func (a *xrayProxy) Protocol() model.Protocol { return a.proto }
func (a *xrayProxy) Engine() string           { return "xray" }

func (a *xrayProxy) Validate(in *model.Inbound) error {
	if in.Port < 1 || in.Port > 65535 {
		return fmt.Errorf("invalid port")
	}
	switch a.proto {
	case model.ProtoTUN:
		return nil
	default:
		if in.Port == 0 {
			return fmt.Errorf("port required")
		}
	}
	return nil
}

func (a *xrayProxy) ToXrayInbound(in *model.Inbound, clients []model.Client) (map[string]any, error) {
	settings := ParseSettings(in.Settings)
	stream := ParseStream(in.StreamSettings)
	sniff := ParseSettings(in.Sniffing)
	if len(sniff) == 0 {
		sniff = DefaultSniffing()
	}

	switch a.proto {
	case model.ProtoVLESS:
		settings["clients"] = buildVLESSClients(clients)
		settings["decryption"] = "none"
	case model.ProtoVMess:
		settings["clients"] = buildVMessClients(clients)
	case model.ProtoTrojan:
		settings["clients"] = buildTrojanClients(clients)
	case model.ProtoShadowsocks:
		if _, ok := settings["method"]; !ok {
			settings["method"] = "aes-256-gcm"
		}
		settings["clients"] = buildSSClients(clients, settings)
	case model.ProtoHTTP, model.ProtoSOCKS:
		settings["accounts"] = buildAccounts(clients)
	case model.ProtoWireGuard, model.ProtoAmneziaWG:
		// peers live in settings; amnezia extras stay in settings as-is
		if a.proto == model.ProtoAmneziaWG {
			settings["amnezia"] = true
		}
	case model.ProtoTunnel:
		if _, ok := settings["address"]; !ok {
			settings["address"] = []string{"localhost"}
		}
	case model.ProtoTUN:
		// TUN settings opaque
	}

	protoName := string(a.proto)
	if a.proto == model.ProtoAmneziaWG {
		protoName = "wireguard"
	}
	if a.proto == model.ProtoTunnel {
		protoName = "dokodemo-door"
	}

	obj := map[string]any{
		"tag":            in.Tag,
		"listen":         in.Listen,
		"port":           in.Port,
		"protocol":       protoName,
		"settings":       settings,
		"streamSettings": stream,
		"sniffing":       sniff,
	}
	if a.proto == model.ProtoTUN {
		delete(obj, "port")
		delete(obj, "listen")
	}
	return obj, nil
}

func buildVLESSClients(clients []model.Client) []map[string]any {
	out := make([]map[string]any, 0, len(clients))
	for _, c := range clients {
		if !c.Enable {
			continue
		}
		m := map[string]any{"id": c.UUID, "email": c.Email}
		if c.Flow != "" {
			m["flow"] = c.Flow
		}
		if c.LimitIP > 0 {
			m["limitIp"] = c.LimitIP
		}
		out = append(out, m)
	}
	return out
}

func buildVMessClients(clients []model.Client) []map[string]any {
	out := make([]map[string]any, 0, len(clients))
	for _, c := range clients {
		if !c.Enable {
			continue
		}
		m := map[string]any{
			"id": c.UUID, "email": c.Email, "alterId": 0,
		}
		if c.LimitIP > 0 {
			m["limitIp"] = c.LimitIP
		}
		out = append(out, m)
	}
	return out
}

func buildTrojanClients(clients []model.Client) []map[string]any {
	out := make([]map[string]any, 0, len(clients))
	for _, c := range clients {
		if !c.Enable {
			continue
		}
		pw := c.Password
		if pw == "" {
			pw = c.UUID
		}
		m := map[string]any{"password": pw, "email": c.Email}
		if c.LimitIP > 0 {
			m["limitIp"] = c.LimitIP
		}
		out = append(out, m)
	}
	return out
}

func buildSSClients(clients []model.Client, settings map[string]any) []map[string]any {
	method, _ := settings["method"].(string)
	out := make([]map[string]any, 0, len(clients))
	for _, c := range clients {
		if !c.Enable {
			continue
		}
		pw := c.Password
		if pw == "" {
			pw = c.UUID
		}
		m := map[string]any{
			"password": pw, "email": c.Email, "method": method,
		}
		if c.LimitIP > 0 {
			m["limitIp"] = c.LimitIP
		}
		out = append(out, m)
	}
	return out
}

func buildAccounts(clients []model.Client) []map[string]any {
	out := make([]map[string]any, 0, len(clients))
	for _, c := range clients {
		if !c.Enable {
			continue
		}
		user := c.Email
		if user == "" {
			user = c.UUID
		}
		pw := c.Password
		if pw == "" {
			pw = c.UUID
		}
		out = append(out, map[string]any{"user": user, "pass": pw})
	}
	return out
}

func (a *xrayProxy) ShareLink(in *model.Inbound, c model.Client, host string) (string, error) {
	stream := ParseStream(in.StreamSettings)
	network, _ := stream["network"].(string)
	security, _ := stream["security"].(string)
	if network == "" {
		network = "tcp"
	}
	if security == "" {
		security = "none"
	}

	switch a.proto {
	case model.ProtoVLESS:
		return vlessLink(in, c, host, network, security, stream), nil
	case model.ProtoVMess:
		return vmessLink(in, c, host, network, security, stream), nil
	case model.ProtoTrojan:
		return trojanLink(in, c, host, network, security, stream), nil
	case model.ProtoShadowsocks:
		return ssLink(in, c, host), nil
	default:
		return fmt.Sprintf("%s://%s@%s:%d#%s", a.proto, c.UUID, host, in.Port, url.QueryEscape(c.Email)), nil
	}
}

func vlessLink(in *model.Inbound, c model.Client, host, network, security string, stream map[string]any) string {
	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("type", network)
	q.Set("security", security)
	if c.Flow != "" {
		q.Set("flow", c.Flow)
	}
	applyStreamQuery(q, stream, security, network)
	name := c.Email
	if name == "" {
		name = in.Remark
	}
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", c.UUID, host, in.Port, q.Encode(), url.QueryEscape(name))
}

func vmessLink(in *model.Inbound, c model.Client, host, network, security string, stream map[string]any) string {
	obj := map[string]any{
		"v": "2", "ps": c.Email, "add": host, "port": fmt.Sprintf("%d", in.Port),
		"id": c.UUID, "aid": "0", "scy": "auto", "net": network, "tls": security,
	}
	if ws, ok := stream["wsSettings"].(map[string]any); ok {
		if p, ok := ws["path"].(string); ok {
			obj["path"] = p
		}
	}
	b, _ := json.Marshal(obj)
	return "vmess://" + base64.StdEncoding.EncodeToString(b)
}

func trojanLink(in *model.Inbound, c model.Client, host, network, security string, stream map[string]any) string {
	pw := c.Password
	if pw == "" {
		pw = c.UUID
	}
	q := url.Values{}
	q.Set("type", network)
	q.Set("security", security)
	applyStreamQuery(q, stream, security, network)
	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", pw, host, in.Port, q.Encode(), url.QueryEscape(c.Email))
}

func ssLink(in *model.Inbound, c model.Client, host string) string {
	settings := ParseSettings(in.Settings)
	method, _ := settings["method"].(string)
	if method == "" {
		method = "aes-256-gcm"
	}
	pw := c.Password
	if pw == "" {
		pw = c.UUID
	}
	userinfo := base64.RawURLEncoding.EncodeToString([]byte(method + ":" + pw))
	return fmt.Sprintf("ss://%s@%s:%d#%s", userinfo, host, in.Port, url.QueryEscape(c.Email))
}

func applyStreamQuery(q url.Values, stream map[string]any, security, network string) {
	if security == "reality" {
		if rs, ok := stream["realitySettings"].(map[string]any); ok {
			if sni, ok := rs["serverNames"].([]any); ok && len(sni) > 0 {
				q.Set("sni", fmt.Sprint(sni[0]))
			}
			if pbk, ok := rs["publicKey"].(string); ok {
				q.Set("pbk", pbk)
			}
			if sid, ok := rs["shortIds"].([]any); ok && len(sid) > 0 {
				q.Set("sid", fmt.Sprint(sid[0]))
			}
			if fp, ok := rs["fingerprint"].(string); ok {
				q.Set("fp", fp)
			}
			q.Set("spx", "/")
		}
	}
	if security == "tls" {
		if ts, ok := stream["tlsSettings"].(map[string]any); ok {
			if sni, ok := ts["serverName"].(string); ok {
				q.Set("sni", sni)
			}
			if fp, ok := ts["fingerprint"].(string); ok {
				q.Set("fp", fp)
			}
			if alpn, ok := ts["alpn"].([]any); ok {
				parts := make([]string, 0, len(alpn))
				for _, a := range alpn {
					parts = append(parts, fmt.Sprint(a))
				}
				q.Set("alpn", strings.Join(parts, ","))
			}
		}
	}
	switch network {
	case "ws":
		if ws, ok := stream["wsSettings"].(map[string]any); ok {
			if p, ok := ws["path"].(string); ok {
				q.Set("path", p)
			}
			if h, ok := ws["headers"].(map[string]any); ok {
				if host, ok := h["Host"].(string); ok {
					q.Set("host", host)
				}
			}
		}
	case "grpc":
		if gs, ok := stream["grpcSettings"].(map[string]any); ok {
			if sn, ok := gs["serviceName"].(string); ok {
				q.Set("serviceName", sn)
			}
		}
	case "httpupgrade":
		if hs, ok := stream["httpupgradeSettings"].(map[string]any); ok {
			if p, ok := hs["path"].(string); ok {
				q.Set("path", p)
			}
		}
	case "xhttp":
		if xs, ok := stream["xhttpSettings"].(map[string]any); ok {
			if p, ok := xs["path"].(string); ok {
				q.Set("path", p)
			}
			if m, ok := xs["mode"].(string); ok {
				q.Set("mode", m)
			}
		}
	case "kcp":
		if ks, ok := stream["kcpSettings"].(map[string]any); ok {
			if seed, ok := ks["seed"].(string); ok {
				q.Set("seed", seed)
			}
		}
	}
}
