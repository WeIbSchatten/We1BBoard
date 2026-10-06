package controller

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/tgnotify"
)

const pending2FAKey = "pending2faSecret"

func settingTruthy(v string) bool { return tgnotify.SettingTruthy(v) }

// Setup2FA generates a new TOTP secret + otpauth URI (stored in session until enable).
// GET /settings/2fa/setup
func (a *API) Setup2FA(c *gin.Context) {
	sess := sessions.Default(c)
	username, _ := sess.Get("username").(string)
	if username == "" {
		username = "admin"
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "We1BBoard",
		AccountName: username,
	})
	if err != nil {
		fail(c, 500, err)
		return
	}
	secret := key.Secret()
	uri := key.URL()
	sess.Set(pending2FAKey, secret)
	_ = sess.Save()

	qrData := ""
	if png, err := qrcode.Encode(uri, qrcode.Medium, 220); err == nil {
		qrData = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	}
	ok(c, gin.H{
		"secret":  secret,
		"otpauth": uri,
		"qr":      qrData,
	})
}

// Enable2FA verifies a TOTP code against the pending secret and persists settings.
// POST /settings/2fa/enable { "code": "123456" }
func (a *API) Enable2FA(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required,max=16"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	sess := sessions.Default(c)
	secret, _ := sess.Get(pending2FAKey).(string)
	if secret == "" {
		fail(c, 400, fmt.Errorf("run 2FA setup first"))
		return
	}
	if !totp.Validate(strings.TrimSpace(req.Code), secret) {
		fail(c, 400, fmt.Errorf("invalid two-factor code"))
		return
	}
	if err := database.SetSetting("twoFactorSecret", secret); err != nil {
		fail(c, 500, err)
		return
	}
	if err := database.SetSetting("twoFactorEnable", "true"); err != nil {
		fail(c, 500, err)
		return
	}
	sess.Delete(pending2FAKey)
	_ = sess.Save()
	ok(c, gin.H{"enabled": true})
}

// Disable2FA verifies TOTP (and optional password) then clears 2FA settings.
// POST /settings/2fa/disable { "code": "123456", "password": "..." }
func (a *API) Disable2FA(c *gin.Context) {
	var req struct {
		Code     string `json:"code" binding:"required,max=16"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, 400, err)
		return
	}
	if !settingTruthy(database.GetSetting("twoFactorEnable")) {
		fail(c, 400, fmt.Errorf("two-factor is not enabled"))
		return
	}
	secret := database.GetSetting("twoFactorSecret")
	if secret == "" || !totp.Validate(strings.TrimSpace(req.Code), secret) {
		fail(c, 400, fmt.Errorf("invalid two-factor code"))
		return
	}
	if strings.TrimSpace(req.Password) != "" {
		sess := sessions.Default(c)
		username, _ := sess.Get("username").(string)
		if username == "" {
			fail(c, 401, fmt.Errorf("unauthorized"))
			return
		}
		if _, err := a.Auth.Login(username, req.Password); err != nil {
			fail(c, 400, fmt.Errorf("invalid password"))
			return
		}
	}
	_ = database.SetSetting("twoFactorEnable", "false")
	_ = database.SetSetting("twoFactorSecret", "")
	sess := sessions.Default(c)
	sess.Delete(pending2FAKey)
	_ = sess.Save()
	ok(c, gin.H{"enabled": false})
}

// TestTelegram sends a test message via the panel bot.
// POST /settings/tg-test { "tgBotToken"?: "...", "tgBotChatId"?: "..." }
func (a *API) TestTelegram(c *gin.Context) {
	var req struct {
		TgBotToken  string `json:"tgBotToken"`
		TgBotChatId string `json:"tgBotChatId"`
	}
	_ = c.ShouldBindJSON(&req)
	token := strings.TrimSpace(req.TgBotToken)
	chatID := strings.TrimSpace(req.TgBotChatId)
	if token == "***" || strings.Contains(token, "…") {
		token = ""
	}
	if err := tgnotify.Test(token, chatID); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, gin.H{"sent": true})
}
