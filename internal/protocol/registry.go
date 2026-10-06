package protocol

import (
	"encoding/json"
	"fmt"

	"github.com/we1bboard/we1bboard/internal/database/model"
)

// Adapter converts panel inbound/client data to engine configs and share links.
type Adapter interface {
	Protocol() model.Protocol
	Validate(in *model.Inbound) error
	Engine() string // xray | mtg | tuic | hysteria2
	ToXrayInbound(in *model.Inbound, clients []model.Client) (map[string]any, error)
	ShareLink(in *model.Inbound, c model.Client, host string) (string, error)
}

var registry = map[model.Protocol]Adapter{}

func Register(a Adapter) {
	registry[a.Protocol()] = a
}

func Get(p model.Protocol) (Adapter, error) {
	a, ok := registry[p]
	if !ok {
		return nil, fmt.Errorf("protocol %s not registered", p)
	}
	return a, nil
}

func All() []model.Protocol {
	out := make([]model.Protocol, 0, len(registry))
	for p := range registry {
		out = append(out, p)
	}
	return out
}

func MustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func ParseSettings(raw string) map[string]any {
	m := map[string]any{}
	if raw == "" {
		return m
	}
	_ = json.Unmarshal([]byte(raw), &m)
	return m
}

func ParseStream(raw string) map[string]any {
	m := map[string]any{
		"network":  "tcp",
		"security": "none",
	}
	if raw == "" {
		return m
	}
	_ = json.Unmarshal([]byte(raw), &m)
	return m
}

func DefaultSniffing() map[string]any {
	return map[string]any{
		"enabled":      true,
		"destOverride": []string{"http", "tls", "quic"},
		"routeOnly":    false,
	}
}

func init() {
	Register(&xrayProxy{proto: model.ProtoVLESS})
	Register(&xrayProxy{proto: model.ProtoVMess})
	Register(&xrayProxy{proto: model.ProtoTrojan})
	Register(&xrayProxy{proto: model.ProtoShadowsocks})
	Register(&xrayProxy{proto: model.ProtoHTTP})
	Register(&xrayProxy{proto: model.ProtoSOCKS})
	Register(&xrayProxy{proto: model.ProtoTunnel})
	Register(&xrayProxy{proto: model.ProtoTUN})
	Register(&xrayProxy{proto: model.ProtoWireGuard})
	Register(&xrayProxy{proto: model.ProtoAmneziaWG})
	Register(&mtprotoAdapter{})
	Register(&tuicAdapter{})
	Register(&hy2Adapter{})
}
