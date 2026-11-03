package httpapi

import (
	"context"
	"sync"
	"time"

	usecaseaudit "geoatlas/internal/usecase/auditlog"
)

// rateLimitCoalesce — at most one audit row per key per minute; details.count accumulates.
type rateLimitCoalesce struct {
	mu   sync.Mutex
	last map[string]time.Time
	cnt  map[string]int
}

func newRateLimitCoalesce() *rateLimitCoalesce {
	return &rateLimitCoalesce{
		last: make(map[string]time.Time),
		cnt:  make(map[string]int),
	}
}

var globalRateLimitAudit = newRateLimitCoalesce()

func (c *rateLimitCoalesce) shouldEmit(key string, now time.Time) (emit bool, count int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cnt[key]++
	count = c.cnt[key]
	last, ok := c.last[key]
	if !ok || now.Sub(last) >= time.Minute {
		c.last[key] = now
		c.cnt[key] = 0
		return true, count
	}
	return false, count
}

func auditSecurityReject(ctx context.Context, logs *usecaseaudit.Service, action, ip, path, reason string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	if reason != "" {
		details["reason"] = reason
	}
	if path != "" {
		details["path"] = path
	}
	writeAuditEvent(ctx, logs, usecaseaudit.AuditEvent{
		Actor:        ip,
		Action:       action,
		ResourceType: "http",
		ResourceID:   path,
		Result:       "failed",
		IP:           ip,
		Details:      details,
	})
}

func auditAPIRateLimit(ctx context.Context, logs *usecaseaudit.Service, rIP, path string) {
	key := rIP
	if key == "" {
		key = "unknown"
	}
	emit, count := globalRateLimitAudit.shouldEmit(key, time.Now())
	if !emit {
		return
	}
	auditSecurityReject(ctx, logs, "security.api_rate_limit", rIP, path, "rate_limited", map[string]any{
		"count": count,
	})
}
