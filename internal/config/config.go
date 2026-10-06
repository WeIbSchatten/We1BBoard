package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Version is overwritten at release build via -ldflags.
var Version = "1.0.0"

type Config struct {
	DataDir    string
	DBPath     string
	DBType     string // sqlite | postgres
	DBDSN      string
	BinDir     string
	XrayBin    string
	MtgBin     string
	TUICBin    string
	Hy2Bin     string
	TgProxyBin string
	LogLevel   string
	Listen     string
}

func Load() *Config {
	dataDir := env("WE1B_DATA_DIR", defaultDataDir())
	binDir := env("WE1B_BIN_DIR", filepath.Join(dataDir, "bin"))
	return &Config{
		DataDir:    dataDir,
		DBPath:     env("WE1B_DB_PATH", filepath.Join(dataDir, "we1bboard.db")),
		DBType:     GetDBKind(),
		DBDSN:      GetDBDSN(),
		BinDir:     binDir,
		XrayBin:    env("WE1B_XRAY_BIN", filepath.Join(binDir, xrayName())),
		MtgBin:     env("WE1B_MTG_BIN", filepath.Join(binDir, mtgName())),
		TUICBin:    env("WE1B_TUIC_BIN", filepath.Join(binDir, "tuic-server")),
		Hy2Bin:     env("WE1B_HY2_BIN", filepath.Join(binDir, "hysteria")),
		TgProxyBin: env("WE1B_TGPROXY_BIN", filepath.Join(binDir, "tproxy-server")),
		LogLevel:   env("WE1B_LOG_LEVEL", "info"),
		Listen:     env("WE1B_LISTEN", ""),
	}
}

// GetDBKind returns sqlite (default) or postgres — mirrors 3x-ui XUI_DB_TYPE.
func GetDBKind() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("WE1B_DB_TYPE")))
	switch v {
	case "postgres", "postgresql", "pg":
		return "postgres"
	default:
		return "sqlite"
	}
}

// GetDBDSN returns PostgreSQL DSN from WE1B_DB_DSN (empty for sqlite).
func GetDBDSN() string {
	return strings.TrimSpace(os.Getenv("WE1B_DB_DSN"))
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Env is the exported alias of env for other packages.
func Env(key, def string) string { return env(key, def) }

func EnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func defaultDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".we1bboard")
	}
	return "./data"
}
