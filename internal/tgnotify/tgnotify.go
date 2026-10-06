package tgnotify

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
)

const telegramHost = "api.telegram.org"

// SettingTruthy parses common truthy setting values.
func SettingTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

// ValidBotToken allows Telegram bot tokens (digits:alphanum/_-).
func ValidBotToken(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 128 {
		return false
	}
	colon := strings.IndexByte(token, ':')
	if colon < 1 {
		return false
	}
	for i, c := range token {
		if i == colon {
			continue
		}
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}

// ValidChatID allows numeric chat IDs (optionally negative for groups) or @channel usernames.
func ValidChatID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 64 {
		return false
	}
	if strings.HasPrefix(id, "@") {
		name := id[1:]
		if name == "" {
			return false
		}
		for _, c := range name {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
				continue
			}
			return false
		}
		return true
	}
	start := 0
	if id[0] == '-' {
		start = 1
	}
	if start >= len(id) {
		return false
	}
	for _, c := range id[start:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func telegramClient() *http.Client {
	base := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	tr := &http.Transport{
		Proxy: nil, // never honor proxy for bot API (SSRF / MITM surface)
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			if !strings.EqualFold(host, telegramHost) {
				return nil, fmt.Errorf("host not allowed")
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, telegramHost)
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

// SendMessage posts to api.telegram.org only (hard-coded host; SSRF-safe).
func SendMessage(token, chatID, text string) error {
	if !ValidBotToken(token) {
		return fmt.Errorf("invalid bot token")
	}
	if !ValidChatID(chatID) {
		return fmt.Errorf("invalid chat id")
	}
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 4000 {
		return fmt.Errorf("invalid message")
	}

	u := url.URL{
		Scheme: "https",
		Host:   telegramHost,
		Path:   "/bot" + token + "/sendMessage",
	}
	body, _ := json.Marshal(map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	})
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Host = telegramHost

	resp, err := telegramClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("telegram bad response")
	}
	if !parsed.OK {
		if parsed.Description != "" {
			return fmt.Errorf("telegram: %s", parsed.Description)
		}
		return fmt.Errorf("telegram request failed")
	}
	return nil
}

// Enabled reports whether the panel Telegram bot is turned on with credentials.
func Enabled() bool {
	if !SettingTruthy(database.GetSetting("tgBotEnable")) {
		return false
	}
	return ValidBotToken(database.GetSetting("tgBotToken")) && ValidChatID(database.GetSetting("tgBotChatId"))
}

// NotifyLogin sends a login alert when tgNotifyLogin is enabled.
func NotifyLogin(username, ip string) {
	if !Enabled() || !SettingTruthy(database.GetSetting("tgNotifyLogin")) {
		return
	}
	token := database.GetSetting("tgBotToken")
	chatID := database.GetSetting("tgBotChatId")
	msg := fmt.Sprintf("We1BBoard login\nuser: %s\nip: %s\ntime: %s",
		username, ip, time.Now().UTC().Format(time.RFC3339))
	_ = SendMessage(token, chatID, msg)
}

// Test sends a test message using current settings (or overrides from the request).
func Test(token, chatID string) error {
	if token == "" {
		token = database.GetSetting("tgBotToken")
	}
	if chatID == "" {
		chatID = database.GetSetting("tgBotChatId")
	}
	return SendMessage(token, chatID, "We1BBoard Telegram test OK")
}
