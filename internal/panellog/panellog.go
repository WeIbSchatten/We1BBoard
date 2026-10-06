package panellog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu   sync.Mutex
	dir  string
	inited bool
)

// Init sets the data directory for panel.log (idempotent).
func Init(dataDir string) {
	mu.Lock()
	defer mu.Unlock()
	dir = dataDir
	inited = dataDir != ""
	if inited {
		_ = os.MkdirAll(dataDir, 0o755)
	}
}

// Path returns {dataDir}/panel.log, or empty if not initialized.
func Path() string {
	mu.Lock()
	defer mu.Unlock()
	if !inited || dir == "" {
		return ""
	}
	return filepath.Join(dir, "panel.log")
}

// Append writes a timestamped line to panel.log. Best-effort; ignores errors.
func Append(format string, args ...any) {
	path := Path()
	if path == "" {
		return
	}
	msg := fmt.Sprintf(format, args...)
	line := time.Now().Format("2006/01/02 15:04:05") + " " + msg + "\n"
	mu.Lock()
	defer mu.Unlock()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(line)
	_ = f.Close()
}
