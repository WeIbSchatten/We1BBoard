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
		Logger:                                   logger.Default.LogMode(logger.Warn),
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
		&model.Outbound{},
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

func seed(db *gorm.DB) error {
	var count int64
	db.Model(&model.User{}).Count(&count)
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := db.Create(&model.User{Username: "admin", PasswordHash: string(hash)}).Error; err != nil {
			return err
		}
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
		"theme":         "night",
		"accent":       "blue",
		"lang":         "ru",
		"xrayTemplate": "",
		"certFile":     "",
		"keyFile":      "",
		"secret":       mustRandomSecret(),
		"nodeToken":    mustRandomSecret(),
		"trafficCron":  "@every 10s",
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

func GetSetting(key string) string {
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
	"secret":    true,
	"nodeToken": true,
}

func AllSettingsPublic() (map[string]string, error) {
	var rows []model.Setting
	if err := DB.Find(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		if SensitiveSettings[r.Key] {
			m[r.Key] = "***"
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
	"subPort": true, "subPath": true, "subEnable": true, "subHost": true, "subTitle": true, "subSupportUrl": true,
	"theme": true, "accent": true, "lang": true,
	"xrayTemplate": true, "certFile": true, "keyFile": true,
	"trafficCron": true,
}
