package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/we1bboard/we1bboard/internal/config"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/extra"
	"github.com/we1bboard/we1bboard/internal/panellog"
	"github.com/we1bboard/we1bboard/internal/tgproxy"
	"github.com/we1bboard/we1bboard/internal/ufw"
	"github.com/we1bboard/we1bboard/internal/web"
	"github.com/we1bboard/we1bboard/internal/web/job"
	"github.com/we1bboard/we1bboard/internal/web/service"
	"github.com/we1bboard/we1bboard/internal/xray"
)

func main() {
	if len(os.Args) < 2 {
		runMenu()
		return
	}
	switch os.Args[1] {
	case "run", "start":
		runServer()
	case "menu":
		runMenu()
	case "setting":
		runSetting(os.Args[2:])
	case "cert":
		runCert(os.Args[2:])
	case "version":
		fmt.Println(config.Version)
	case "reset-admin":
		if len(os.Args) >= 4 {
			ensureDB()
			auth := &service.AuthService{}
			if err := auth.ResetAdmin(os.Args[2], os.Args[3]); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			fmt.Println("admin reset ok")
			return
		}
		resetAdmin()
	default:
		fmt.Println("We1BBoard commands: run | menu | setting | cert | reset-admin | version")
	}
}

func runServer() {
	cfg := config.Load()
	_ = os.MkdirAll(cfg.DataDir, 0o755)
	_ = os.MkdirAll(cfg.BinDir, 0o755)
	panellog.Init(cfg.DataDir)
	panellog.Append("panel starting v%s", config.Version)
	if err := database.Init(cfg); err != nil {
		fmt.Println("db error:", err)
		panellog.Append("db init failed: %v", err)
		os.Exit(1)
	}
	xrayMgr := xray.NewManager(cfg.XrayBin, filepath.Join(cfg.DataDir, "xray"))
	extraMgr := extra.NewManager(cfg.MtgBin, cfg.TUICBin, cfg.Hy2Bin, filepath.Join(cfg.DataDir, "extra"))
	tgMgr := tgproxy.NewManager(cfg.TgProxyBin, filepath.Join(cfg.DataDir, "tgproxy"))

	// Best-effort start of engines (binaries may be absent during UI-only boot)
	_ = xrayMgr.WriteConfig()
	if err := xrayMgr.Start(); err != nil {
		fmt.Println("xray:", err)
	}
	_ = extraMgr.SyncAll()
	tgMgr.StartEnabled()

	srv := web.NewServer(cfg, xrayMgr, extraMgr, tgMgr)
	job.Start(srv.RT)
	go func() {
		ufw.SyncAllInbounds()
	}()

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		<-ch
		_ = xrayMgr.Stop()
		os.Exit(0)
	}()

	if err := srv.Start(); err != nil {
		fmt.Println("server error:", err)
		os.Exit(1)
	}
}

func runMenu() {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println()
		fmt.Println("========== We1BBoard ==========")
		fmt.Println("1. Start panel")
		fmt.Println("2. Show settings")
		fmt.Println("3. Set panel port")
		fmt.Println("4. Set panel path")
		fmt.Println("5. Reset admin password")
		fmt.Println("6. Set theme (light/night/amoled)")
		fmt.Println("7. Set accent (blue/purple)")
		fmt.Println("8. Show certificate paths")
		fmt.Println("0. Exit")
		fmt.Print("Select: ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		switch line {
		case "1":
			runServer()
			return
		case "2":
			ensureDB()
			m, _ := database.AllSettings()
			for k, v := range m {
				if k == "secret" || k == "nodeToken" {
					v = "***"
				}
				fmt.Printf("  %s = %s\n", k, v)
			}
		case "3":
			ensureDB()
			fmt.Print("Port: ")
			p, _ := reader.ReadString('\n')
			_ = database.SetSetting("panelPort", strings.TrimSpace(p))
		case "4":
			ensureDB()
			fmt.Print("Path (e.g. /we1b/): ")
			p, _ := reader.ReadString('\n')
			_ = database.SetSetting("panelPath", strings.TrimSpace(p))
		case "5":
			resetAdmin()
		case "6":
			ensureDB()
			fmt.Print("Theme: ")
			p, _ := reader.ReadString('\n')
			_ = database.SetSetting("theme", strings.TrimSpace(p))
		case "7":
			ensureDB()
			fmt.Print("Accent: ")
			p, _ := reader.ReadString('\n')
			_ = database.SetSetting("accent", strings.TrimSpace(p))
		case "8":
			ensureDB()
			fmt.Printf("certFile=%s\nkeyFile=%s\n", database.GetSetting("certFile"), database.GetSetting("keyFile"))
		case "0":
			return
		}
	}
}

func ensureDB() {
	cfg := config.Load()
	if database.DB == nil {
		if err := database.Init(cfg); err != nil {
			fmt.Println("db:", err)
			os.Exit(1)
		}
	}
}

func resetAdmin() {
	ensureDB()
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Username [admin]: ")
	u, _ := reader.ReadString('\n')
	u = strings.TrimSpace(u)
	if u == "" {
		u = "admin"
	}
	fmt.Print("New password: ")
	p, _ := reader.ReadString('\n')
	p = strings.TrimSpace(p)
	if p == "" {
		fmt.Println("password required")
		return
	}
	auth := &service.AuthService{}
	if err := auth.ResetAdmin(u, p); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("admin reset ok")
}

func runSetting(args []string) {
	ensureDB()
	if len(args) < 1 {
		fmt.Println("usage: setting get|set <key> [value]")
		return
	}
	switch args[0] {
	case "get":
		if len(args) < 2 {
			return
		}
		fmt.Println(database.GetSetting(args[1]))
	case "set":
		if len(args) < 3 {
			return
		}
		_ = database.SetSetting(args[1], args[2])
	case "port":
		if len(args) < 2 {
			return
		}
		if _, err := strconv.Atoi(args[1]); err == nil {
			_ = database.SetSetting("panelPort", args[1])
		}
	}
}

// runCert: we1bboard cert -webCert /path/fullchain.pem -webCertKey /path/privkey.pem
func runCert(args []string) {
	ensureDB()
	var cert, key string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-webCert", "--webCert", "-cert":
			if i+1 < len(args) {
				i++
				cert = args[i]
			}
		case "-webCertKey", "--webCertKey", "-key":
			if i+1 < len(args) {
				i++
				key = args[i]
			}
		}
	}
	if cert == "" || key == "" {
		fmt.Println("usage: we1bboard cert -webCert /path/fullchain.pem -webCertKey /path/privkey.pem")
		os.Exit(1)
	}
	if _, err := os.Stat(cert); err != nil {
		fmt.Println("cert file:", err)
		os.Exit(1)
	}
	if _, err := os.Stat(key); err != nil {
		fmt.Println("key file:", err)
		os.Exit(1)
	}
	_ = database.SetSetting("certFile", cert)
	_ = database.SetSetting("keyFile", key)
	fmt.Println("certificate paths saved; restart panel to apply")
}
