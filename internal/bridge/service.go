package bridge

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
)

// XrayOutboundProtocols — dialer protocols supported for bridge chaining.
var XrayOutboundProtocols = []string{
	"vless", "vmess", "trojan", "shadowsocks", "socks", "http", "wireguard",
}

type Service struct{}

func NewService() *Service { return &Service{} }

func (s *Service) List() ([]model.Bridge, error) {
	var rows []model.Bridge
	err := database.DB.Order("id desc").Find(&rows).Error
	return rows, err
}

func (s *Service) Get(id uint) (*model.Bridge, error) {
	var b model.Bridge
	if err := database.DB.First(&b, id).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Service) Create(b *model.Bridge) error {
	if b.Name == "" {
		return fmt.Errorf("name required")
	}
	if b.DialerProtocol == "" {
		b.DialerProtocol = "vless"
	}
	if !validProto(b.DialerProtocol) {
		return fmt.Errorf("unsupported dialer protocol %q (use xray outbound: vless/vmess/trojan/ss/socks/http/wireguard, or set dialerSettings JSON)", b.DialerProtocol)
	}
	needsID := b.DialerProtocol == "vless" || b.DialerProtocol == "vmess"
	if needsID && b.DialerUUID == "" {
		b.DialerUUID = uuid.NewString()
	}
	if b.OutboundTag == "" {
		b.OutboundTag = fmt.Sprintf("bridge-%s", b.Name)
	}
	if b.DialerNetwork == "" {
		b.DialerNetwork = "tcp"
	}
	if b.DialerSecurity == "" {
		b.DialerSecurity = "none"
	}
	if b.DialerFingerprt == "" {
		b.DialerFingerprt = "chrome"
	}
	if b.DialerSettings == "" {
		b.DialerSettings = "{}"
	}
	if b.DialerStreamSettings == "" {
		b.DialerStreamSettings = "{}"
	}
	return database.DB.Create(b).Error
}

func (s *Service) Update(b *model.Bridge) error {
	if b.DialerProtocol != "" && !validProto(b.DialerProtocol) {
		return fmt.Errorf("unsupported dialer protocol %q", b.DialerProtocol)
	}
	return database.DB.Save(b).Error
}

func (s *Service) Delete(id uint) error {
	return database.DB.Delete(&model.Bridge{}, id).Error
}

func validProto(p string) bool {
	for _, x := range XrayOutboundProtocols {
		if x == p {
			return true
		}
	}
	// allow any string if full settings JSON provided (advanced)
	return true
}

// BuildDialerHint returns a suggested EU-side accept inbound matching the bridge dialer.
func (s *Service) BuildDialerHint(b *model.Bridge) map[string]any {
	proto := b.DialerProtocol
	if proto == "" {
		proto = "vless"
	}
	hint := map[string]any{
		"protocol": proto,
		"port":     b.DialerPort,
		"stream": map[string]any{
			"network":  nullable(b.DialerNetwork, "tcp"),
			"security": nullable(b.DialerSecurity, "none"),
		},
		"note": "Create a matching inbound on the exit (EU) node, then point this bridge dialer at it. Any Xray outbound protocol is supported; for full control fill dialerSettings + dialerStreamSettings JSON.",
	}
	switch proto {
	case "vless", "vmess":
		hint["clients"] = []map[string]any{{"id": b.DialerUUID, "flow": b.DialerFlow}}
	case "trojan", "shadowsocks":
		hint["clients"] = []map[string]any{{"password": first(b.DialerPassword, b.DialerUUID)}}
	case "socks", "http":
		hint["accounts"] = []map[string]any{{"user": b.DialerEmail, "pass": b.DialerPassword}}
	}
	if b.DialerSecurity == "reality" {
		hint["stream"].(map[string]any)["realitySettings"] = map[string]any{
			"dest":        b.DialerSNI + ":443",
			"serverNames": []string{b.DialerSNI},
			"shortIds":    []string{b.DialerShortID},
			"publicKey":   b.DialerPublicKey,
		}
	}
	return hint
}

func nullable(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
