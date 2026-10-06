package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/we1bboard/we1bboard/internal/config"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

const (
	DialectSQLite   = "sqlite"
	DialectPostgres = "postgres"
)

func IsPostgres() bool {
	if DB == nil {
		return config.GetDBKind() == DialectPostgres
	}
	return DB.Name() == DialectPostgres
}

func Dialect() string {
	if DB == nil {
		return ""
	}
	return DB.Name()
}

// Init opens SQLite or PostgreSQL (WE1B_DB_TYPE / WE1B_DB_DSN), like 3x-ui.
func Init(cfg *config.Config) error {
	gormCfg := &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Error),
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	var (
		db  *gorm.DB
		err error
	)

	switch config.GetDBKind() {
	case DialectPostgres:
		dsn := config.GetDBDSN()
		if dsn == "" {
			return errors.New("WE1B_DB_TYPE=postgres but WE1B_DB_DSN is empty")
		}
		db, err = openPostgresWithRetry(dsn, gormCfg)
		if err != nil {
			return err
		}
	default:
		dbPath := cfg.DBPath
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
			return fmt.Errorf("create db dir: %w", err)
		}
		if err := applyPendingRestore(cfg); err != nil {
			log.Printf("restore apply: %v", err)
		}
		dsn := dbPath + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
		db, err = gorm.Open(sqlite.Open(dsn), gormCfg)
		if err != nil {
			return fmt.Errorf("open sqlite: %w", err)
		}
		_ = restrictFilePerms(dbPath)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	maxOpen, maxIdle := 8, 4
	if config.GetDBKind() == DialectPostgres {
		maxOpen = config.EnvInt("WE1B_DB_MAX_OPEN_CONNS", 25)
		maxIdle = config.EnvInt("WE1B_DB_MAX_IDLE_CONNS", 25)
	} else {
		maxOpen = config.EnvInt("WE1B_DB_MAX_OPEN_CONNS", 8)
		maxIdle = config.EnvInt("WE1B_DB_MAX_IDLE_CONNS", 4)
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	if err := db.AutoMigrate(
		&model.User{},
		&model.Setting{},
		&model.Inbound{},
		&model.Client{},
		&model.ClientIP{},
		&model.ClientHWID{},
		&model.Host{},
		&model.Outbound{},
		&model.OutboundSubscription{},
		&model.SubBalancer{},
		&model.Node{},
		&model.Bridge{},
		&model.TgProxyProfile{},
		&model.RoutingRule{},
	); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	DB = db
	return seed(db)
}

func openPostgresWithRetry(dsn string, c *gorm.Config) (*gorm.DB, error) {
	delays := []time.Duration{0, 2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second}
	var lastErr error
	for i, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}
		conn, err := gorm.Open(postgres.Open(dsn), c)
		if err == nil {
			if i > 0 {
				log.Printf("postgres connection established on attempt %d/%d", i+1, len(delays))
			}
			sqlDB, err := conn.DB()
			if err != nil {
				return nil, err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = sqlDB.PingContext(ctx)
			cancel()
			if err != nil {
				lastErr = err
				log.Printf("postgres ping attempt %d/%d failed: %v", i+1, len(delays), err)
				continue
			}
			return conn, nil
		}
		lastErr = err
		log.Printf("postgres connection attempt %d/%d failed: %v", i+1, len(delays), err)
	}
	return nil, fmt.Errorf("postgres unreachable after %d attempts: %w", len(delays), lastErr)
}

func restrictFilePerms(path string) error {
	return os.Chmod(path, 0o600)
}

// applyPendingRestore renames {DataDir}/we1bboard.db.restoring over DBPath before open.
func applyPendingRestore(cfg *config.Config) error {
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = filepath.Dir(cfg.DBPath)
	}
	pending := filepath.Join(dataDir, "we1bboard.db.restoring")
	st, err := os.Stat(pending)
	if err != nil || st.IsDir() || st.Size() < 100 {
		return nil
	}
	dbPath := cfg.DBPath
	bak := dbPath + ".pre-restore"
	_ = os.Remove(bak)
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Rename(dbPath, bak); err != nil {
			return fmt.Errorf("backup current db: %w", err)
		}
	}
	if err := os.Rename(pending, dbPath); err != nil {
		_ = os.Rename(bak, dbPath) // best-effort rollback
		return fmt.Errorf("apply restore: %w", err)
	}
	_ = restrictFilePerms(dbPath)
	log.Printf("applied pending database restore from %s", pending)
	return nil
}

func seed(db *gorm.DB) error {
	var count int64
	db.Model(&model.User{}).Count(&count)
	if count == 0 {
		pass := mustRandomPassword(16)
		hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := db.Create(&model.User{Username: "admin", PasswordHash: string(hash)}).Error; err != nil {
			return err
		}
		writeInstallCredentials("admin", pass)
		log.Printf("We1BBoard: initial admin user created (credentials in %s/install-result.env)", envDataDir())
	}
	defaults := map[string]string{
		"panelPort":    "2053",
		"panelPath":    "/we1b/",
		"webListen":    "0.0.0.0",
		"subPort":      "2096",
		"subPath":      "/sub/",
		"subEnable":    "true",
		"subHost":      "",
		"subTitle":      "We1BBoard",
		"subSupportUrl": "",
		"subThemeDir":   "",
		"subAnnounce":   "",
		"subJsonEnable": "true",
		"subClashEnable": "true",
		"clientGroups":  "[]",
		"ufwEnable":     "true",
		"theme":         "night",
		"accent":       "blue",
		"lang":         "ru",
		"xrayTemplate": "",
		"certFile":     "",
		"keyFile":      "",
		"secret":       mustRandomSecret(),
		"nodeToken":    mustRandomSecret(),
		"twoFactorEnable": "false",
		"twoFactorSecret": "",
		"tgBotEnable":     "false",
		"tgBotToken":      "",
		"tgBotChatId":     "",
		"tgNotifyLogin":   "false",
		"tgNotifyTraffic": "false",
		"emailEnable":          "false",
		"smtpHost":             "",
		"smtpPort":             "587",
		"smtpUser":             "",
		"smtpPass":             "",
		"smtpFrom":             "",
		"emailNotifyLogin":     "false",
		"emailNotifyTraffic":   "false",
		"discordEnable":        "false",
		"discordWebhook":       "",
		"discordNotifyLogin":   "false",
		"discordNotifyTraffic": "false",
		"trafficCron":            "@every 10s",
		"routingDomainStrategy":  "AsIs",
		"geodataGeositeURL": "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat",
		"geodataGeoipURL":   "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat",
	}
	for k, v := range defaults {
		var s model.Setting
		err := db.Where(&model.Setting{Key: k}).First(&s).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&model.Setting{Key: k, Value: v}).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	var outCount int64
	db.Model(&model.Outbound{}).Count(&outCount)
	if outCount == 0 {
		outs := []model.Outbound{
			{Tag: "direct", Protocol: "freedom", Settings: `{}`, Enable: true, Remark: "Direct"},
			{Tag: "blocked", Protocol: "blackhole", Settings: `{}`, Enable: true, Remark: "Block"},
		}
		for i := range outs {
			if err := db.Create(&outs[i]).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func mustRandomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable")
	}
	return hex.EncodeToString(b)
}

func mustRandomPassword(n int) string {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable")
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(out)
}

func writeInstallCredentials(user, pass string) {
	dir := envDataDir()
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "install-result.env")
	body := fmt.Sprintf("WE1B_USERNAME=%q\nWE1B_PASSWORD=%q\n", user, pass)
	_ = os.WriteFile(path, []byte(body), 0o600)
}

func envDataDir() string {
	dir := os.Getenv("WE1B_DATA_DIR")
	if dir == "" {
		dir = "/etc/we1bboard"
	}
	return dir
}

func GetSetting(key string) string {
	if DB == nil {
		return ""
	}
	var s model.Setting
	if err := DB.Where(&model.Setting{Key: key}).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

func SetSetting(key, value string) error {
	var s model.Setting
	err := DB.Where(&model.Setting{Key: key}).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DB.Create(&model.Setting{Key: key, Value: value}).Error
	}
	if err != nil {
		return err
	}
	s.Value = value
	return DB.Save(&s).Error
}

// SensitiveSettings must never be returned to the browser.
var SensitiveSettings = map[string]bool{
	"secret":          true,
	"nodeToken":       true,
	"twoFactorSecret": true,
	"tgBotToken":      true,
	"smtpPass":        true,
	"discordWebhook":  true,
}

func AllSettingsPublic() (map[string]string, error) {
	var rows []model.Setting
	if err := DB.Find(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		if SensitiveSettings[r.Key] {
			if r.Value == "" {
				m[r.Key] = ""
			} else {
				m[r.Key] = "***"
			}
			continue
		}
		m[r.Key] = r.Value
	}
	return m, nil
}

func AllSettings() (map[string]string, error) {
	var rows []model.Setting
	if err := DB.Find(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Key] = r.Value
	}
	return m, nil
}

// AllowedSettingKeys — whitelist for panel UI updates (secrets excluded).
var AllowedSettingKeys = map[string]bool{
	"panelPort": true, "panelPath": true, "webListen": true,
	"subPort": true, "subPath": true, "subEnable": true, "subHost": true, "subTitle": true, "subSupportUrl": true, "subThemeDir": true, "subAnnounce": true,
	"subJsonEnable": true, "subClashEnable": true,
	"clientGroups": true,
	"ufwEnable": true,
	"theme": true, "accent": true, "lang": true,
	"xrayTemplate": true, "certFile": true, "keyFile": true,
	"trafficCron": true,
	"routingDomainStrategy": true,
	"geodataGeositeURL": true,
	"geodataGeoipURL":   true,
	"twoFactorEnable": true, "twoFactorSecret": true,
	"tgBotEnable": true, "tgBotToken": true, "tgBotChatId": true,
	"tgNotifyLogin": true, "tgNotifyTraffic": true,
	"emailEnable": true, "smtpHost": true, "smtpPort": true, "smtpUser": true, "smtpPass": true, "smtpFrom": true,
	"emailNotifyLogin": true, "emailNotifyTraffic": true,
	"discordEnable": true, "discordWebhook": true, "discordNotifyLogin": true, "discordNotifyTraffic": true,
}
