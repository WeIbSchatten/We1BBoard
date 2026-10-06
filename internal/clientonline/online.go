package clientonline

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
	"github.com/we1bboard/we1bboard/internal/panellog"
	"github.com/we1bboard/we1bboard/internal/xray"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OnlineWindow is how long a lastSeen stays "online".
const OnlineWindow = 2 * time.Minute

// RecentIPWindow is how long an IP counts toward LimitIP warnings.
const RecentIPWindow = 2 * time.Minute

var (
	mu       sync.RWMutex
	lastSeen = map[string]int64{} // email -> unix ms
)

// Touch records activity for email at unixMs (or now if unixMs <= 0).
func Touch(email string, unixMs int64) {
	email = strings.TrimSpace(email)
	if email == "" {
		return
	}
	if unixMs <= 0 {
		unixMs = time.Now().UnixMilli()
	}
	mu.Lock()
	if prev, ok := lastSeen[email]; !ok || unixMs > prev {
		lastSeen[email] = unixMs
	}
	mu.Unlock()
}

// Snapshot returns currently-online emails and the full lastSeen map (filtered to online).
func Snapshot() (emails []string, m map[string]int64) {
	cutoff := time.Now().Add(-OnlineWindow).UnixMilli()
	mu.RLock()
	defer mu.RUnlock()
	m = make(map[string]int64, len(lastSeen))
	emails = make([]string, 0, len(lastSeen))
	for email, ts := range lastSeen {
		if ts >= cutoff {
			m[email] = ts
			emails = append(emails, email)
		}
	}
	return emails, m
}

// IsOnline reports whether email was seen within OnlineWindow.
func IsOnline(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	cutoff := time.Now().Add(-OnlineWindow).UnixMilli()
	mu.RLock()
	ts, ok := lastSeen[email]
	mu.RUnlock()
	return ok && ts >= cutoff
}

// LastSeenMs returns lastSeen unix ms for email, or 0.
func LastSeenMs(email string) int64 {
	mu.RLock()
	defer mu.RUnlock()
	return lastSeen[strings.TrimSpace(email)]
}

// AccessEntry is a parsed xray access.log line with email.
type AccessEntry struct {
	Email string
	IP    string
}

// ParseAccessLine extracts email and source IP from an xray access log line.
// Example: `2024/01/02 15:04:05.123456 from 1.2.3.4:555 accepted tcp:… email: alice@example.com`
func ParseAccessLine(line string) (AccessEntry, bool) {
	parts := strings.Fields(line)
	var entry AccessEntry
	for i, part := range parts {
		if part == "from" && i+1 < len(parts) {
			addr := strings.TrimLeft(parts[i+1], "/")
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				host = addr
			}
			entry.IP = host
		} else if part == "email:" && i+1 < len(parts) {
			entry.Email = parts[i+1]
		} else if strings.HasPrefix(part, "email:") && len(part) > len("email:") {
			entry.Email = strings.TrimPrefix(part, "email:")
		}
	}
	if entry.Email == "" {
		return AccessEntry{}, false
	}
	return entry, true
}

type emailIP struct{ email, ip string }

// SampleAccessLog reads the last nLines of path and updates online + ClientIP rows.
func SampleAccessLog(path string, nLines int) {
	if path == "" || nLines <= 0 {
		return
	}
	lines, err := xray.TailFile(path, nLines)
	if err != nil || len(lines) == 0 {
		return
	}
	now := time.Now().UnixMilli()
	seen := map[emailIP]int64{}
	touched := map[string]bool{}
	for _, line := range lines {
		if !strings.Contains(line, "email:") {
			continue
		}
		e, ok := ParseAccessLine(line)
		if !ok {
			continue
		}
		Touch(e.Email, now)
		touched[e.Email] = true
		if e.IP == "" || e.IP == "127.0.0.1" || e.IP == "::1" {
			continue
		}
		k := emailIP{e.Email, e.IP}
		if prev, exists := seen[k]; !exists || now > prev {
			seen[k] = now
		}
	}
	if database.DB == nil {
		return
	}
	for k, ts := range seen {
		upsertClientIP(k.email, k.ip, ts)
	}
	warnLimitIP(touched)
}

func upsertClientIP(email, ip string, lastSeenMs int64) {
	row := model.ClientIP{Email: email, IP: ip, LastSeen: lastSeenMs}
	_ = database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}, {Name: "ip"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_seen"}),
	}).Create(&row).Error
}

func warnLimitIP(emails map[string]bool) {
	if len(emails) == 0 {
		return
	}
	list := make([]string, 0, len(emails))
	for e := range emails {
		list = append(list, e)
	}
	var clients []model.Client
	if err := database.DB.Select("email", "limit_ip").Where("email IN ? AND limit_ip > 0", list).Find(&clients).Error; err != nil {
		return
	}
	if len(clients) == 0 {
		return
	}
	cutoff := time.Now().Add(-RecentIPWindow).UnixMilli()
	for _, c := range clients {
		var ips []model.ClientIP
		_ = database.DB.Where("email = ? AND last_seen >= ?", c.Email, cutoff).Find(&ips).Error
		uniq := map[string]bool{}
		for _, row := range ips {
			uniq[row.IP] = true
		}
		if len(uniq) > c.LimitIP {
			panellog.Append("LimitIP warning: email=%s ips=%d limit=%d (kick skipped)", c.Email, len(uniq), c.LimitIP)
		}
	}
}

// ListIPs returns stored IPs for email ordered by lastSeen desc.
func ListIPs(email string) ([]model.ClientIP, error) {
	var rows []model.ClientIP
	err := database.DB.Where("email = ?", email).Order("last_seen desc").Find(&rows).Error
	return rows, err
}

// ClearIPs deletes all IP rows for email.
func ClearIPs(email string) error {
	return database.DB.Where("email = ?", email).Delete(&model.ClientIP{}).Error
}

// ListHWIDs returns HWID rows for email.
func ListHWIDs(email string) ([]model.ClientHWID, error) {
	var rows []model.ClientHWID
	err := database.DB.Where("email = ?", email).Order("id asc").Find(&rows).Error
	return rows, err
}

// AddHWID registers a hwid for email if not already present.
func AddHWID(email, hwid string) (*model.ClientHWID, error) {
	email = strings.TrimSpace(email)
	hwid = strings.TrimSpace(hwid)
	if email == "" || hwid == "" {
		return nil, fmt.Errorf("email and hwid required")
	}
	if len(hwid) > 128 {
		return nil, fmt.Errorf("hwid too long")
	}
	var existing model.ClientHWID
	err := database.DB.Where("email = ? AND hwid = ?", email, hwid).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	row := model.ClientHWID{Email: email, HWID: hwid, CreatedAt: time.Now()}
	if err := database.DB.Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// DeleteHWID deletes one HWID row by id for email.
func DeleteHWID(email string, id uint) error {
	res := database.DB.Where("email = ? AND id = ?", email, id).Delete(&model.ClientHWID{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("hwid not found")
	}
	return nil
}

// ClearHWIDs deletes all HWIDs for email.
func ClearHWIDs(email string) error {
	return database.DB.Where("email = ?", email).Delete(&model.ClientHWID{}).Error
}

// EnforceHWIDForClients applies LimitHWID rules for the given clients (same sub).
// Missing header is allowed. New HWIDs under the limit are auto-registered.
// Returns false when the request must be rejected with 403.
func EnforceHWIDForClients(clients []model.Client, hwid string) bool {
	hwid = strings.TrimSpace(hwid)
	if hwid == "" {
		return true
	}
	maxLimit := 0
	emails := make([]string, 0, len(clients))
	for _, c := range clients {
		if c.LimitHWID > maxLimit {
			maxLimit = c.LimitHWID
		}
		if c.Email != "" {
			emails = append(emails, c.Email)
		}
	}
	if maxLimit <= 0 || len(emails) == 0 {
		return true
	}
	// Prefer primary client email (first) for storage.
	primary := emails[0]

	var existing []model.ClientHWID
	_ = database.DB.Where("email IN ?", emails).Find(&existing).Error
	for _, row := range existing {
		if row.HWID == hwid {
			return true
		}
	}
	if len(existing) >= maxLimit {
		return false
	}
	_, err := AddHWID(primary, hwid)
	return err == nil
}
