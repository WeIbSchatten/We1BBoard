// Package xrayinstall downloads Xray-core releases (Linux amd64/arm64).
package xrayinstall

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/we1bboard/we1bboard/internal/config"
	"github.com/we1bboard/we1bboard/internal/security"
)

const maxArchiveBytes = 200 << 20 // 200 MiB

// Result is returned after a successful install.
type Result struct {
	Version string
	Bin     string
	Bytes   int64
}

// InstallLatest downloads Xray-core into destBin (default: binDir/xray).
// version may be empty (latest), "1.8.24", or "v1.8.24".
func InstallLatest(binDir, destBin, version string) (*Result, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("xray install not supported on %s (linux only)", runtime.GOOS)
	}
	if binDir == "" {
		return nil, fmt.Errorf("bin dir not configured")
	}
	if destBin == "" {
		destBin = filepath.Join(binDir, "xray")
	}

	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	client := security.SafeHTTPClient(false, 120*time.Second)
	if version == "" {
		latest, err := fetchLatestTag(client)
		if err != nil {
			return nil, err
		}
		version = strings.TrimPrefix(latest, "v")
	}
	asset, err := assetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/v%s/%s", version, asset)
	if err := security.ValidateNodeURL(url); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return nil, err
	}
	tmpArc := filepath.Join(binDir, asset+".tmp")
	n, err := downloadToFile(client, url, tmpArc)
	if err != nil {
		_ = os.Remove(tmpArc)
		return nil, fmt.Errorf("download: %w", err)
	}
	defer os.Remove(tmpArc)

	tmpBin := destBin + ".new"
	if err := extractBinary(tmpArc, asset, tmpBin); err != nil {
		_ = os.Remove(tmpBin)
		return nil, fmt.Errorf("extract: %w", err)
	}
	_ = os.Chmod(tmpBin, 0o755)
	if err := os.Rename(tmpBin, destBin); err != nil {
		_ = os.Remove(tmpBin)
		return nil, err
	}
	_ = os.Chmod(destBin, 0o755)

	return &Result{Version: version, Bin: destBin, Bytes: n}, nil
}

// EnsureInstalled installs latest Xray-core when destBin is missing (Linux only).
// Returns (true, nil) if a download was performed.
func EnsureInstalled(binDir, destBin string) (downloaded bool, res *Result, err error) {
	if destBin == "" {
		destBin = filepath.Join(binDir, "xray")
	}
	if _, err := os.Stat(destBin); err == nil {
		return false, nil, nil
	}
	if runtime.GOOS != "linux" {
		return false, nil, fmt.Errorf("xray binary missing (%s)", destBin)
	}
	res, err = InstallLatest(binDir, destBin, "")
	if err != nil {
		return false, nil, err
	}
	return true, res, nil
}

func fetchLatestTag(client *http.Client) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/XTLS/Xray-core/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "We1BBoard/"+config.DisplayVersion())
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

func assetName(goos, goarch string) (string, error) {
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

func downloadToFile(client *http.Client, rawURL, dest string) (int64, error) {
	resp, err := client.Get(rawURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxArchiveBytes+1))
	_ = f.Close()
	if err != nil {
		_ = os.Remove(dest)
		return 0, err
	}
	if n > maxArchiveBytes {
		_ = os.Remove(dest)
		return 0, fmt.Errorf("file exceeds %d bytes", maxArchiveBytes)
	}
	if n < 100 {
		_ = os.Remove(dest)
		return 0, fmt.Errorf("file too small")
	}
	return n, nil
}

func extractBinary(archivePath, assetName, destBin string) error {
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
	_, err = io.Copy(out, io.LimitReader(rc, maxArchiveBytes))
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
		_, copyErr := io.Copy(out, io.LimitReader(tr, maxArchiveBytes))
		_ = out.Close()
		return copyErr
	}
	return fmt.Errorf("%s not found in tarball", wantName)
}
