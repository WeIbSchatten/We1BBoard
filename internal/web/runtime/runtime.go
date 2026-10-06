package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/extra"
	"github.com/we1bboard/we1bboard/internal/panellog"
	"github.com/we1bboard/we1bboard/internal/security"
	"github.com/we1bboard/we1bboard/internal/tgproxy"
	"github.com/we1bboard/we1bboard/internal/xray"
)

// Runtime applies config changes to local processes or remote nodes.
type Runtime interface {
	Reload() error
	Kind() string
}

type Local struct {
	Xray    *xray.Manager
	Extra   *extra.Manager
	TgProxy *tgproxy.Manager
}

func (l *Local) Kind() string { return "local" }

func (l *Local) Reload() error {
	var first error
	if l.Xray != nil {
		if err := l.Xray.WriteConfig(); err != nil && first == nil {
			first = err
			panellog.Append("reload: WriteConfig failed: %v", err)
		}
		if _, err := os.Stat(l.Xray.Bin); err != nil {
			if first == nil {
				panellog.Append("reload: xray binary missing (%s)", l.Xray.Bin)
			}
			if l.Extra != nil {
				_ = l.Extra.SyncAll()
			}
			return first
		}
		if l.Xray.IsRunning() {
			if err := l.Xray.Reload(); err != nil && first == nil {
				first = err
				panellog.Append("reload: Xray.Reload failed: %v", err)
			}
		} else {
			if err := l.Xray.Start(); err != nil && first == nil {
				first = err
				panellog.Append("reload: Xray.Start failed: %v", err)
			}
		}
	}
	if l.Extra != nil {
		_ = l.Extra.SyncAll()
	}
	return first
}

type Remote struct {
	Node *model.Node
}

func (r *Remote) Kind() string { return "remote" }

func (r *Remote) client() *http.Client {
	return security.SafeHTTPClient(r.Node.TLSMode == "skip", 15*time.Second)
}

func (r *Remote) Reload() error {
	return r.post("/panel/api/xray/restart", nil)
}

func (r *Remote) post(path string, body any) error {
	if err := security.ValidateNodeURL(r.Node.URL); err != nil {
		return err
	}
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	}
	url := strings.TrimRight(r.Node.URL, "/") + path
	req, err := http.NewRequest(http.MethodPost, url, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.Node.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote %s: %s", resp.Status, string(b))
	}
	return nil
}

func (r *Remote) Heartbeat() error {
	if err := security.ValidateNodeURL(r.Node.URL); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(r.Node.URL, "/")+"/panel/api/server/status", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.Node.Token)
	resp, err := r.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("remote status %s", resp.Status)
	}
	return nil
}

type Hub struct {
	Local *Local
}

func NewHub(local *Local) *Hub { return &Hub{Local: local} }

func (h *Hub) ForNode(nodeID *uint) Runtime {
	if nodeID == nil || *nodeID == 0 {
		return h.Local
	}
	var n model.Node
	if err := database.DB.First(&n, *nodeID).Error; err != nil {
		return h.Local
	}
	return &Remote{Node: &n}
}

func (h *Hub) ReloadLocal() error { return h.Local.Reload() }

func (h *Hub) PingNodes() {
	var nodes []model.Node
	_ = database.DB.Where("enable = ?", true).Find(&nodes)
	for i := range nodes {
		n := &nodes[i]
		r := &Remote{Node: n}
		online := r.Heartbeat() == nil
		_ = database.DB.Model(n).Updates(map[string]any{
			"online":    online,
			"last_seen": time.Now().Unix(),
		})
	}
}
