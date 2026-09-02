package threatprot

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter — per-key token bucket (Apigee SpikeArrest equivalent).
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
	ttl      time.Duration
	lastSeen map[string]time.Time
}

// NewRateLimiter creates a limiter. rps<=0 disables limiting.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	if burst <= 0 {
		burst = int(rps)
		if burst < 1 {
			burst = 1
		}
	}
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
		ttl:      10 * time.Minute,
		lastSeen: make(map[string]time.Time),
	}
}

func (l *RateLimiter) Allow(key string) bool {
	if l == nil || l.rps <= 0 || key == "" {
		return true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gcLocked(now)
	lim := l.limiters[key]
	if lim == nil {
		lim = rate.NewLimiter(l.rps, l.burst)
		l.limiters[key] = lim
	}
	l.lastSeen[key] = now
	return lim.Allow()
}

func (l *RateLimiter) gcLocked(now time.Time) {
	if len(l.lastSeen) < 2048 {
		return
	}
	for k, at := range l.lastSeen {
		if now.Sub(at) > l.ttl {
			delete(l.lastSeen, k)
			delete(l.limiters, k)
		}
	}
}

// RateLimitKey returns the client identity for SpikeArrest (IP or Bearer prefix).
func RateLimitKey(r *http.Request, clientIP func(*http.Request) string) string {
	if r == nil {
		return ""
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		tok := strings.TrimSpace(auth[len("Bearer "):])
		if len(tok) > 16 {
			tok = tok[:16]
		}
		if tok != "" {
			return "tok:" + tok
		}
	}
	return "ip:" + clientIP(r)
}
