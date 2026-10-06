package controller

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/we1bboard/we1bboard/internal/config"
	"github.com/we1bboard/we1bboard/internal/panellog"
	"github.com/we1bboard/we1bboard/internal/security"
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
	if destBin == "" {
		destBin = filepath.Join(binDir, "xray")
	}

	version := strings.TrimSpace(strings.TrimPrefix(req.Version, "v"))
	client := security.SafeHTTPClient(false, 120*time.Second)
	if version == "" {
		latest, err := fetchLatestXrayTag(client)
		if err != nil {
			fail(c, 502, err)
			return
		}
		version = strings.TrimPrefix(latest, "v")
	}
	asset, err := xrayAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		fail(c, 400, err)
		return
	}
	url := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/v%s/%s", version, asset)
	if err := security.ValidateNodeURL(url); err != nil {
		fail(c, 400, err)
		return
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		fail(c, 500, err)
		return
	}
	tmpArc := filepath.Join(binDir, asset+".tmp")
	n, err := downloadToFile(client, url, tmpArc)
	if err != nil {
		_ = os.Remove(tmpArc)
		fail(c, 502, fmt.Errorf("download: %w", err))
		return
	}
	defer os.Remove(tmpArc)

	tmpBin := destBin + ".new"
	if err := extractXrayBinary(tmpArc, asset, tmpBin); err != nil {
		fail(c, 500, fmt.Errorf("extract: %w", err))
		return
	}
	_ = os.Chmod(tmpBin, 0o755)
	if err := os.Rename(tmpBin, destBin); err != nil {
		_ = os.Remove(tmpBin)
		fail(c, 500, err)
		return
	}
	_ = os.Chmod(destBin, 0o755)

	if a.Xray != nil {
		a.Xray.Bin = destBin
		_ = a.Xray.Stop()
		_ = a.Xray.WriteConfig()
		if err := a.Xray.Start(); err != nil {
			panellog.Append("install-xray: restart failed: %v", err)
			fail(c, 500, fmt.Errorf("installed but restart failed: %w", err))
			return
		}
	}
	panellog.Append("install-xray: v%s (%d bytes) -> %s", version, n, destBin)
	current := ""
	if out, err := exec.Command(destBin, "version").CombinedOutput(); err == nil {
		current = parseXrayVersion(string(out))
	}
	ok(c, gin.H{
		"version": version,
		"current": current,
		"bin":     destBin,
		"bytes":   n,
		"message": "xray installed and restarted",
	})
}

func fetchLatestXrayTag(client *http.Client) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/XTLS/Xray-core/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "We1BBoard/"+config.Version)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("github status %d", resp.StatusCode)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", err
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("empty tag")
	}
	return rel.TagName, nil
}

func xrayAssetName(goos, goarch string) (string, error) {
	if goos != "linux" {
		return "", fmt.Errorf("unsupported OS %s (linux only)", goos)
	}
	switch goarch {
	case "amd64":
		return "Xray-linux-64.zip", nil
	case "arm64":
		return "Xray-linux-arm64-v8a.zip", nil
	default:
		return "", fmt.Errorf("unsupported arch %s", goarch)
	}
}

func extractXrayBinary(archivePath, assetName, destBin string) error {
	lower := strings.ToLower(assetName)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZipNamed(archivePath, destBin, "xray")
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return extractTarGzNamed(archivePath, destBin, "xray")
	default:
		return fmt.Errorf("unknown archive type: %s", assetName)
	}
}

func extractZipNamed(zipPath, dest, wantName string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	var found *zip.File
	for i := range r.File {
		f := r.File[i]
		base := filepath.Base(f.Name)
		if base == wantName || base == wantName+".exe" {
			found = f
			break
		}
	}
	if found == nil {
		return fmt.Errorf("%s not found in zip", wantName)
	}
	rc, err := found.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, io.LimitReader(rc, 200<<20))
	return err
}

func extractTarGzNamed(path, dest, wantName string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) != wantName {
			continue
		}
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, io.LimitReader(tr, 200<<20))
		_ = out.Close()
		return copyErr
	}
	return fmt.Errorf("%s not found in tarball", wantName)
}
