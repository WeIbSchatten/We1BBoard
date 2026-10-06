package protocol

import (
	"fmt"

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
		"port":           in.Port,
		"protocol":       protoName,
		"settings":       settings,
		"streamSettings": stream,
		"sniffing":       sniff,
	}
	if in.Listen != "" {
		obj["listen"] = in.Listen
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

// ShareLink is implemented in share.go (3x-ui-compatible URI builders).
