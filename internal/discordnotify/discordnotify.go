package discordnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/tgnotify"
)

var allowedHosts = map[string]bool{
	"discord.com":    true,
	"discordapp.com": true,
}

// ValidWebhookURL accepts only https Discord webhook URLs (SSRF-safe).
func ValidWebhookURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 512 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if !allowedHosts[host] {
		return false
	}
	path := u.EscapedPath()
	return strings.HasPrefix(path, "/api/webhooks/")
}

// Enabled reports whether Discord notify is on with a valid webhook.
func Enabled() bool {
	if !tgnotify.SettingTruthy(database.GetSetting("discordEnable")) {
		return false
	}
	return ValidWebhookURL(database.GetSetting("discordWebhook"))
}

func discordClient(host string) *http.Client {
	base := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	tr := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			h, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if !strings.EqualFold(h, host) || !allowedHosts[strings.ToLower(host)] {
				return nil, fmt.Errorf("host not allowed")
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(ips) == 0 {
				return nil, fmt.Errorf("resolve failed")
			}
			var last error
			for _, ipa := range ips {
				conn, err := base.DialContext(ctx, network, net.JoinHostPort(ipa.IP.String(), port))
				if err == nil {
					return conn, nil
				}
				last = err
			}
			if last == nil {
				last = fmt.Errorf("no usable address")
			}
			return nil, last
		},
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return &http.Client{
		Timeout:   15 * time.Second,
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Send posts JSON {content} to the Discord webhook.
func Send(webhookURL, content string) error {
	if !ValidWebhookURL(webhookURL) {
		return fmt.Errorf("invalid discord webhook URL")
	}
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 1900 {
		return fmt.Errorf("invalid message")
	}
	u, err := url.Parse(webhookURL)
	if err != nil {
		return err
	}
	host := strings.ToLower(u.Hostname())
	body, _ := json.Marshal(map[string]string{"content": content})
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Host = host

	resp, err := discordClient(host).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

// NotifyLogin sends a login alert when discordNotifyLogin is enabled.
func NotifyLogin(username, ip string) {
	if !Enabled() || !tgnotify.SettingTruthy(database.GetSetting("discordNotifyLogin")) {
		return
	}
	webhook := database.GetSetting("discordWebhook")
	msg := fmt.Sprintf("We1BBoard login\nuser: %s\nip: %s\ntime: %s",
		username, ip, time.Now().UTC().Format(time.RFC3339))
	_ = Send(webhook, msg)
}

// NotifyTraffic sends a traffic alert when discordNotifyTraffic is enabled.
func NotifyTraffic(text string) {
	if !Enabled() || !tgnotify.SettingTruthy(database.GetSetting("discordNotifyTraffic")) {
		return
	}
	webhook := database.GetSetting("discordWebhook")
	_ = Send(webhook, text)
}

// Test sends a test message using the stored (or override) webhook.
func Test(webhookURL string) error {
	if webhookURL == "" || webhookURL == "***" || strings.Contains(webhookURL, "…") {
		webhookURL = database.GetSetting("discordWebhook")
	}
	return Send(webhookURL, "We1BBoard Discord test OK")
}
