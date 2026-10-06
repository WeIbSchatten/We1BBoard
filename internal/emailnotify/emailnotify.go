package emailnotify

import (
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/tgnotify"
)

// Enabled reports whether SMTP email notify is configured and turned on.
func Enabled() bool {
	if !tgnotify.SettingTruthy(database.GetSetting("emailEnable")) {
		return false
	}
	host := strings.TrimSpace(database.GetSetting("smtpHost"))
	from := strings.TrimSpace(database.GetSetting("smtpFrom"))
	return host != "" && from != "" && ValidSMTPHost(host)
}

// ValidSMTPHost rejects empty / oversized hosts and obvious SSRF (raw IPs still allowed for LAN SMTP).
func ValidSMTPHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" || len(host) > 255 {
		return false
	}
	if strings.ContainsAny(host, " \t\r\n/:@") {
		return false
	}
	return true
}

func smtpPort() int {
	p, err := strconv.Atoi(strings.TrimSpace(database.GetSetting("smtpPort")))
	if err != nil || p < 1 || p > 65535 {
		return 587
	}
	return p
}

// Send delivers a plain-text email via net/smtp.
func Send(to, subject, body string) error {
	host := strings.TrimSpace(database.GetSetting("smtpHost"))
	if !ValidSMTPHost(host) {
		return fmt.Errorf("invalid smtpHost")
	}
	from := strings.TrimSpace(database.GetSetting("smtpFrom"))
	if from == "" || len(from) > 256 {
		return fmt.Errorf("invalid smtpFrom")
	}
	to = strings.TrimSpace(to)
	if to == "" {
		to = from
	}
	user := strings.TrimSpace(database.GetSetting("smtpUser"))
	pass := database.GetSetting("smtpPass")
	port := smtpPort()
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	msg := strings.Builder{}
	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + sanitizeHeader(subject) + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, pass, host)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg.String()))
}

func sanitizeHeader(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, s)
}

// NotifyLogin sends a login alert when emailNotifyLogin is enabled.
func NotifyLogin(username, ip string) {
	if !Enabled() || !tgnotify.SettingTruthy(database.GetSetting("emailNotifyLogin")) {
		return
	}
	to := strings.TrimSpace(database.GetSetting("smtpFrom"))
	body := fmt.Sprintf("We1BBoard login\nuser: %s\nip: %s\ntime: %s",
		username, ip, time.Now().UTC().Format(time.RFC3339))
	_ = Send(to, "We1BBoard login", body)
}

// Test sends a test email using current SMTP settings.
func Test() error {
	if !ValidSMTPHost(database.GetSetting("smtpHost")) {
		return fmt.Errorf("smtpHost required")
	}
	from := strings.TrimSpace(database.GetSetting("smtpFrom"))
	if from == "" {
		return fmt.Errorf("smtpFrom required")
	}
	return Send(from, "We1BBoard email test", "We1BBoard SMTP test OK")
}
