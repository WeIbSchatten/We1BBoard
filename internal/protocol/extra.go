package protocol

import (
	"fmt"
	"net/url"

	"github.com/we1bboard/we1bboard/internal/database/model"
)

type mtprotoAdapter struct{}

func (a *mtprotoAdapter) Protocol() model.Protocol { return model.ProtoMTProto }
func (a *mtprotoAdapter) Engine() string           { return "mtg" }

func (a *mtprotoAdapter) Validate(in *model.Inbound) error {
	if in.Port < 1 || in.Port > 65535 {
		return fmt.Errorf("invalid port")
	}
	return nil
}

func (a *mtprotoAdapter) ToXrayInbound(in *model.Inbound, clients []model.Client) (map[string]any, error) {
	return nil, fmt.Errorf("mtproto uses mtg engine, not xray inbound")
}

func (a *mtprotoAdapter) ShareLink(in *model.Inbound, c model.Client, host string) (string, error) {
	secret := c.Password
	if secret == "" {
		settings := ParseSettings(in.Settings)
		if s, ok := settings["secret"].(string); ok {
			secret = s
		}
	}
	return fmt.Sprintf("https://t.me/proxy?server=%s&port=%d&secret=%s",
		url.QueryEscape(host), in.Port, url.QueryEscape(secret)), nil
}

type tuicAdapter struct{}

func (a *tuicAdapter) Protocol() model.Protocol { return model.ProtoTUIC }
func (a *tuicAdapter) Engine() string           { return "tuic" }

func (a *tuicAdapter) Validate(in *model.Inbound) error {
	if in.Port < 1 || in.Port > 65535 {
		return fmt.Errorf("invalid port")
	}
	return nil
}

func (a *tuicAdapter) ToXrayInbound(in *model.Inbound, clients []model.Client) (map[string]any, error) {
	return nil, fmt.Errorf("tuic uses external engine")
}

func (a *tuicAdapter) ShareLink(in *model.Inbound, c model.Client, host string) (string, error) {
	q := url.Values{}
	q.Set("congestion_control", "bbr")
	q.Set("udp_relay_mode", "native")
	q.Set("alpn", "h3")
	return fmt.Sprintf("tuic://%s:%s@%s:%d?%s#%s",
		c.UUID, c.Password, host, in.Port, q.Encode(), url.QueryEscape(c.Email)), nil
}

type hy2Adapter struct{}

func (a *hy2Adapter) Protocol() model.Protocol { return model.ProtoHysteria2 }
func (a *hy2Adapter) Engine() string           { return "hysteria2" }

func (a *hy2Adapter) Validate(in *model.Inbound) error {
	if in.Port < 1 || in.Port > 65535 {
		return fmt.Errorf("invalid port")
	}
	return nil
}

func (a *hy2Adapter) ToXrayInbound(in *model.Inbound, clients []model.Client) (map[string]any, error) {
	return nil, fmt.Errorf("hysteria2 uses external engine")
}

func (a *hy2Adapter) ShareLink(in *model.Inbound, c model.Client, host string) (string, error) {
	pw := c.Password
	if pw == "" {
		pw = c.UUID
	}
	q := url.Values{}
	q.Set("insecure", "0")
	return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s",
		pw, host, in.Port, q.Encode(), url.QueryEscape(c.Email)), nil
}
