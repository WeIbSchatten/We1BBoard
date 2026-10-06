package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/we1bboard/we1bboard/internal/config"
	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/panellog"
	"github.com/we1bboard/we1bboard/internal/security"
	"github.com/we1bboard/we1bboard/internal/web/history"
)

const restoreMaxBytes = 64 << 20 // 64 MiB

// ServerHistory returns the in-memory metrics ring.
// GET /server/history
func (a *API) ServerHistory(c *gin.Context) {
	ok(c, history.Get())
}

// BackupDB downloads the SQLite database file.
// GET /server/backup
func (a *API) BackupDB(c *gin.Context) {
	if database.IsPostgres() || config.GetDBKind() == database.DialectPostgres {
		fail(c, 400, fmt.Errorf("postgres: use pg_dump for backups"))
		return
	}
	path := a.Cfg.DBPath
	if path == "" {
		fail(c, 500, fmt.Errorf("db path not configured"))
		return
	}
	if _, err := os.Stat(path); err != nil {
		fail(c, 404, fmt.Errorf("database file not found"))
		return
	}
	c.Header("Content-Disposition", `attachment; filename="we1bboard-backup.db"`)
	c.Header("Content-Type", "application/octet-stream")
	c.File(path)
}

// RestoreDB accepts a multipart SQLite upload and schedules a restart on Linux.
// POST /server/restore  form field: file
func (a *API) RestoreDB(c *gin.Context) {
	if database.IsPostgres() || config.GetDBKind() == database.DialectPostgres {
		fail(c, 400, fmt.Errorf("postgres: use pg_restore / psql for restores"))
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		fail(c, 400, fmt.Errorf("multipart file required (field name: file)"))
		return
	}
	if fh.Size > restoreMaxBytes {
		fail(c, 400, fmt.Errorf("file too large (max %d bytes)", restoreMaxBytes))
		return
	}
	src, err := fh.Open()
	if err != nil {
		fail(c, 400, err)
		return
	}
	defer src.Close()

	dataDir := a.Cfg.DataDir
	if dataDir == "" {
		dataDir = filepath.Dir(a.Cfg.DBPath)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fail(c, 500, err)
		return
	}
	dest := filepath.Join(dataDir, "we1bboard.db.restoring")
	tmp, err := os.CreateTemp(dataDir, "restore-*.tmp")
	if err != nil {
		fail(c, 500, err)
		return
	}
	tmpName := tmp.Name()
	n, copyErr := io.Copy(tmp, io.LimitReader(src, restoreMaxBytes+1))
	_ = tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpName)
		fail(c, 500, copyErr)
		return
	}
	if n > restoreMaxBytes {
		_ = os.Remove(tmpName)
		fail(c, 400, fmt.Errorf("file too large"))
		return
	}
	if n < 100 {
		_ = os.Remove(tmpName)
		fail(c, 400, fmt.Errorf("file too small to be a database"))
		return
	}
	if err := os.Rename(tmpName, dest); err != nil {
		_ = os.Remove(tmpName)
		fail(c, 500, err)
		return
	}
	_ = os.Chmod(dest, 0o600)
	panellog.Append("restore: saved %s (%d bytes)", dest, n)

	willExit := runtime.GOOS == "linux" || os.Getenv("WE1B_ALLOW_RESTORE_EXIT") == "1"
	msg := "restore file saved; restart the panel to apply"
	if willExit {
		msg = "restore file saved; panel will exit in 1s for systemd restart"
		go func() {
			time.Sleep(1 * time.Second)
			panellog.Append("restore: exiting for restart")
			os.Exit(0)
		}()
	}
	ok(c, gin.H{
		"path":    dest,
		"bytes":   n,
		"willExit": willExit,
		"message": msg,
	})
}

// UpdateInfo fetches the latest GitHub release tag for WeIbSchatten/We1BBoard.
// GET /server/update-info
func (a *API) UpdateInfo(c *gin.Context) {
	client := security.SafeHTTPClient(false, 20*time.Second)
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/WeIbSchatten/We1BBoard/releases/latest", nil)
	if err != nil {
		fail(c, 500, err)
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "We1BBoard/"+config.Version)
	resp, err := client.Do(req)
	if err != nil {
		fail(c, 502, fmt.Errorf("github: %w", err))
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		fail(c, 502, err)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fail(c, 502, fmt.Errorf("github status %d", resp.StatusCode))
		return
	}
	var rel struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		fail(c, 502, fmt.Errorf("github json: %w", err))
		return
	}
	ok(c, gin.H{
		"current": config.Version,
		"latest":  strings.TrimPrefix(rel.TagName, "v"),
		"tag":     rel.TagName,
		"htmlUrl": rel.HTMLURL,
	})
}

// PanelUpdate runs scripts/update.sh on Linux (fire-and-forget).
// POST /server/update
func (a *API) PanelUpdate(c *gin.Context) {
	if runtime.GOOS != "linux" {
		fail(c, 400, fmt.Errorf("use we1bboard update on server"))
		return
	}
	script := findUpdateScript()
	if script == "" {
		fail(c, 400, fmt.Errorf("use we1bboard update on server"))
		return
	}
	panellog.Append("panel update: starting %s", script)
	cmd := exec.Command("bash", script)
	cmd.Dir = filepath.Dir(script)
	if err := cmd.Start(); err != nil {
		fail(c, 500, fmt.Errorf("failed to start update: %w", err))
		return
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			panellog.Append("panel update failed: %v", err)
			return
		}
		panellog.Append("panel update finished ok")
	}()
	ok(c, gin.H{"started": true, "script": script, "message": "update started; check panel.log"})
}

func findUpdateScript() string {
	candidates := []string{
		"/usr/local/we1bboard/update.sh",
		"/usr/local/we1bboard/scripts/update.sh",
		"/etc/we1bboard/update.sh",
	}
	if a, err := os.Executable(); err == nil {
		dir := filepath.Dir(a)
		candidates = append(candidates,
			filepath.Join(dir, "update.sh"),
			filepath.Join(dir, "scripts", "update.sh"),
		)
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "scripts", "update.sh"))
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}
