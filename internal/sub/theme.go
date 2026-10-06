package sub

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/we1bboard/we1bboard/internal/database"
)

const maxThemeFileBytes = 512 << 10 // 512 KiB

var (
	themeMu        sync.Mutex
	themeCache     *template.Template
	themeCachePath string
	themeCacheMod  time.Time
)

func underDir(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if parent == child {
		return true
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(child, parent+sep)
}

// ResolveThemeFile returns absolute path to sub.html or index.html under dir, or "".
// Symlinks that escape the theme directory are rejected.
func ResolveThemeFile(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", nil
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("subThemeDir must be an absolute path")
	}
	if strings.Contains(dir, "\x00") {
		return "", fmt.Errorf("invalid subThemeDir")
	}
	clean := filepath.Clean(dir)
	fi, err := os.Lstat(clean)
	if err != nil {
		return "", fmt.Errorf("subThemeDir not accessible")
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		// allow symlinked theme root only if resolved target is a directory
		resolved, err := filepath.EvalSymlinks(clean)
		if err != nil {
			return "", fmt.Errorf("subThemeDir symlink unresolvable")
		}
		clean = resolved
		fi, err = os.Stat(clean)
		if err != nil {
			return "", fmt.Errorf("subThemeDir not accessible")
		}
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("subThemeDir must be a directory")
	}
	for _, name := range []string{"sub.html", "index.html"} {
		p := filepath.Join(clean, name)
		st, err := os.Lstat(p)
		if err != nil {
			continue
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("theme file must not be a symlink")
		}
		if st.IsDir() {
			continue
		}
		if st.Size() > maxThemeFileBytes {
			return "", fmt.Errorf("theme file too large (max %d bytes)", maxThemeFileBytes)
		}
		// double-check containment after clean
		if !underDir(clean, p) {
			continue
		}
		return p, nil
	}
	return "", fmt.Errorf("no sub.html or index.html in subThemeDir")
}

// ValidateThemeDir checks the setting value (empty = use built-in page).
func ValidateThemeDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}
	_, err := ResolveThemeFile(dir)
	return err
}

func loadCustomTheme() (*template.Template, error) {
	dir := database.GetSetting("subThemeDir")
	path, err := ResolveThemeFile(dir)
	if err != nil || path == "" {
		return nil, err
	}
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("theme file must not be a symlink")
	}
	themeMu.Lock()
	defer themeMu.Unlock()
	if themeCache != nil && themeCachePath == path && themeCacheMod.Equal(st.ModTime()) {
		return themeCache, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// re-check type after open (TOCTOU)
	fi, err := f.Stat()
	if err != nil || fi.IsDir() || fi.Size() > maxThemeFileBytes {
		return nil, fmt.Errorf("invalid theme file")
	}
	raw := make([]byte, fi.Size())
	if _, err := f.Read(raw); err != nil {
		return nil, err
	}
	tmpl, err := template.New(filepath.Base(path)).Option("missingkey=zero").Parse(string(raw))
	if err != nil {
		return nil, err
	}
	themeCache = tmpl
	themeCachePath = path
	themeCacheMod = st.ModTime()
	return themeCache, nil
}

// customPageVM is the 3x-ui-compatible template view-model (lowercase keys via map).
func customPageVM(pd pageData, up, down, total int64) map[string]any {
	remain := int64(0)
	if total > up+down {
		remain = total - (up + down)
	}
	links := pd.Links
	if !pd.Full {
		links = nil // copy-only browser nav never injects configs into custom HTML either
	}
	return map[string]any{
		"sId":           pd.SubID,
		"enabled":       pd.Active,
		"isOnline":      false,
		"download":      pd.Download,
		"upload":        pd.Upload,
		"total":         pd.Total,
		"used":          pd.Used,
		"remained":      pd.Remain,
		"expire":        pd.ExpireUnix,
		"lastOnline":    int64(0),
		"downloadByte":  down,
		"uploadByte":    up,
		"totalByte":     total,
		"subUrl":        pd.SubURL,
		"subJsonUrl":    pd.JSONURL,
		"subClashUrl":   pd.ClashURL,
		"subSingboxUrl": pd.SingboxURL,
		"subTitle":      pd.Title,
		"subSupportUrl": pd.SupportURL,
		"links":         links,
		"emails":        pd.Emails,
		"announce":      database.GetSetting("subAnnounce"),
		"datepicker":    "gregorian",
		"qrDataUrl":     pd.QRDataURL,
		"full":          pd.Full,
		"remainByte":    remain,
		"Title":         pd.Title,
		"SubID":         pd.SubID,
		"Full":          pd.Full,
		"Upload":        pd.Upload,
		"Download":      pd.Download,
		"Total":         pd.Total,
		"Used":          pd.Used,
		"Remain":        pd.Remain,
		"Expire":        pd.Expire,
		"ExpireUnix":    pd.ExpireUnix,
		"Active":        pd.Active,
		"SubURL":        pd.SubURL,
		"ClashURL":      pd.ClashURL,
		"SingboxURL":    pd.SingboxURL,
		"JSONURL":       pd.JSONURL,
		"QRDataURL":     pd.QRDataURL,
		"Links":         links,
		"Emails":        pd.Emails,
		"SupportURL":    pd.SupportURL,
	}
}
