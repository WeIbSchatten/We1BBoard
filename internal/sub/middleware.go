package sub

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func subSecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Cache-Control", "no-store")
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
	go func() {
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
	}()
	return l
}

func (l *ipLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.visitors[ip]
	if !ok {
		// ~2 req/s burst 20 — clients refresh often; still blocks scrapers
		lim := rate.NewLimiter(rate.Limit(2), 20)
		l.visitors[ip] = &visitor{limiter: lim, lastSeen: time.Now()}
		return lim
	}
	v.lastSeen = time.Now()
	return v.limiter
}

var subLimiter = newIPLimiter()

func subRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !subLimiter.get(c.ClientIP()).Allow() {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	}
}

// requireSubEnabled honors subEnable at request time (not only at process start).
func requireSubEnabled() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !Enabled() {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		c.Next()
	}
}
