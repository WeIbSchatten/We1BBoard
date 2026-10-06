package controller

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/we1bboard/we1bboard/internal/panellog"
	"github.com/we1bboard/we1bboard/internal/xrayinstall"
)

var xrayVersionRe = regexp.MustCompile(`(?i)Xray\s+(\d+\.\d+\.\d+(?:[^\s]*)?)`)

// XrayVersion returns the installed xray binary version and path.
// GET /server/xray-version
func (a *API) XrayVersion(c *gin.Context) {
	bin := ""
	if a.Cfg != nil {
		bin = a.Cfg.XrayBin
	}
	if a.Xray != nil && a.Xray.Bin != "" {
		bin = a.Xray.Bin
	}
	current := ""
	if bin != "" {
		if out, err := exec.Command(bin, "version").CombinedOutput(); err == nil {
			current = parseXrayVersion(string(out))
		}
	}
	ok(c, gin.H{"current": current, "bin": bin})
}

func parseXrayVersion(stdout string) string {
	if m := xrayVersionRe.FindStringSubmatch(stdout); len(m) > 1 {
		return m[1]
	}
	// Fallback: first non-empty line
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// InstallXray downloads Xray-core into BinDir and restarts.
// POST /server/install-xray {version?: string}
func (a *API) InstallXray(c *gin.Context) {
	if runtime.GOOS == "windows" {
		fail(c, 400, fmt.Errorf("xray install not supported on Windows"))
		return
	}
	var req struct {
		Version string `json:"version"`
	}
	_ = c.ShouldBindJSON(&req)

	binDir := ""
	if a.Cfg != nil {
		binDir = a.Cfg.BinDir
	}
	if binDir == "" {
		fail(c, 500, fmt.Errorf("bin dir not configured"))
		return
	}
	destBin := ""
	if a.Cfg != nil {
		destBin = a.Cfg.XrayBin
	}

	res, err := xrayinstall.InstallLatest(binDir, destBin, req.Version)
	if err != nil {
		fail(c, 502, err)
		return
	}

	if a.Xray != nil {
		a.Xray.Bin = res.Bin
		_ = a.Xray.Stop()
		_ = a.Xray.WriteConfig()
		if err := a.Xray.Start(); err != nil {
			panellog.Append("install-xray: restart failed: %v", err)
			fail(c, 500, fmt.Errorf("installed but restart failed: %w", err))
			return
		}
	}
	panellog.Append("install-xray: v%s (%d bytes) -> %s", res.Version, res.Bytes, res.Bin)
	current := ""
	if out, err := exec.Command(res.Bin, "version").CombinedOutput(); err == nil {
		current = parseXrayVersion(string(out))
	}
	ok(c, gin.H{
		"version": res.Version,
		"current": current,
		"bin":     res.Bin,
		"bytes":   res.Bytes,
		"message": "xray installed and restarted",
	})
}
