package middleware

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/we1bboard/we1bboard/internal/config"
	"golang.org/x/time/rate"
)

// MaxBodyBytes rejects oversized JSON bodies (DoS).
func MaxBodyBytes(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}

type ipLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newIPLimiter() *ipLimiter {
	l := &ipLimiter{visitors: map[string]*visitor{}}
	go l.cleanup()
	return l
}

func (l *ipLimiter) cleanup() {
	for {
		time.Sleep(time.Minute)
		l.mu.Lock()
		for ip, v := range l.visitors {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

func (l *ipLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.visitors[ip]
	if !ok {
		lim := rate.NewLimiter(rate.Every(12*time.Second), 5)
		l.visitors[ip] = &visitor{limiter: lim, lastSeen: time.Now()}
		return lim
	}
	v.lastSeen = time.Now()
	return v.limiter
}

var loginLimiter = newIPLimiter()

// LoginRateLimit mitigates password brute-force (per IP and per username).
func LoginRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !loginLimiter.get("ip:"+ip).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "too many login attempts, try later",
			})
			return
		}
		// peek username without consuming body — Gin caches ShouldBindJSON body via GetRawData elsewhere;
		// use a light peek from already-parsed JSON if present, else key only by IP above.
		c.Next()
	}
}

// LoginUserLimit applies after JSON bind; call from Login handler with username.
func LoginUserLimit(username string) bool {
	key := "user:" + strings.ToLower(strings.TrimSpace(username))
	if key == "user:" {
		return true
	}
	return loginLimiter.get(key).Allow()
}

// CSRFOriginCheck blocks cross-site state-changing requests for cookie sessions.
// Node bearer tokens skip this (no cookies).
func CSRFOriginCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if strings.HasPrefix(c.GetHeader("Authorization"), "Bearer ") {
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin == "" {
			// non-browser clients / same-origin navigations without Origin
			ref := c.GetHeader("Referer")
			if ref == "" {
				c.Next()
				return
			}
			if u, err := url.Parse(ref); err == nil {
				origin = u.Scheme + "://" + u.Host
			}
		}
		if origin == "" {
			c.Next()
			return
		}
		ou, err := url.Parse(origin)
		if err != nil || ou.Host == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "invalid origin"})
			return
		}
		reqHost := c.Request.Host
		if ou.Host != reqHost {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "csrf origin mismatch"})
			return
		}
		c.Next()
	}
}

// TrustProxy configures Gin client IP / forwarded headers only when explicitly enabled.
// WE1B_TRUSTED_PROXIES is a comma-separated CIDR/IP list; if empty while trust is on,
// only private RFC1918 + loopback ranges are trusted (never "trust all").
func TrustProxy(r *gin.Engine) {
	if config.Env("WE1B_TRUST_PROXY", "") != "1" {
		_ = r.SetTrustedProxies([]string{})
		return
	}
	raw := strings.TrimSpace(config.Env("WE1B_TRUSTED_PROXIES", ""))
	if raw == "" {
		_ = r.SetTrustedProxies([]string{
			"127.0.0.1/32", "::1/128",
			"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
			"fc00::/7",
		})
		return
	}
	parts := strings.Split(raw, ",")
	cidrs := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cidrs = append(cidrs, p)
		}
	}
	_ = r.SetTrustedProxies(cidrs)
}
