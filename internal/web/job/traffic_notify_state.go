package job

import (
	"strings"
	"sync"
)

var (
	traffic80Mu sync.Mutex
	traffic80   = map[string]bool{} // email -> already notified at ≥80%
)

// MarkTraffic80Notified records that email already received an 80% warning this cycle.
func MarkTraffic80Notified(email string) {
	email = strings.TrimSpace(email)
	if email == "" {
		return
	}
	traffic80Mu.Lock()
	traffic80[email] = true
	traffic80Mu.Unlock()
}

// WasTraffic80Notified reports whether email was already warned at 80%.
func WasTraffic80Notified(email string) bool {
	traffic80Mu.Lock()
	defer traffic80Mu.Unlock()
	return traffic80[strings.TrimSpace(email)]
}

// ClearTraffic80Notified clears the 80% warning flag (after reset or drop below threshold).
func ClearTraffic80Notified(email string) {
	email = strings.TrimSpace(email)
	if email == "" {
		return
	}
	traffic80Mu.Lock()
	delete(traffic80, email)
	traffic80Mu.Unlock()
}
