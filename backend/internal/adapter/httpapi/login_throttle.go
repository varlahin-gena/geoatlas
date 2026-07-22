package httpapi

import (
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// loginLimiter — per-IP throttle + аудит неуспешных попыток по паре username+IP.
type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*loginBucket
	failures map[string]*FailedLoginEvent // key: lower(username)|ip
	maxFails int
	window   time.Duration
	lockout  time.Duration
	retain   time.Duration
	maxAudit int
}

type loginBucket struct {
	fails       int
	windowAt    time.Time
	lockedUntil time.Time
}

// FailedLoginEvent — сводка неуспешных попыток входа для UI мониторинга.
type FailedLoginEvent struct {
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	Count     int       `json:"count"`
	FirstAt   time.Time `json:"first_at"`
	LastAt    time.Time `json:"last_at"`
	Locked    bool      `json:"locked"`
	LockedUntil time.Time `json:"locked_until,omitempty"`
}

func newLoginLimiter(maxFails int, window, lockout time.Duration) *loginLimiter {
	if maxFails <= 0 {
		maxFails = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	if lockout <= 0 {
		lockout = 5 * time.Minute
	}
	return &loginLimiter{
		attempts: make(map[string]*loginBucket),
		failures: make(map[string]*FailedLoginEvent),
		maxFails: maxFails,
		window:   window,
		lockout:  lockout,
		retain:   24 * time.Hour,
		maxAudit: 200,
	}
}

var defaultLoginLimiter = newLoginLimiter(10, time.Minute, 5*time.Minute)

// clientIP — IP для throttle/аудита.
// Доверяем только X-Real-IP (его выставляет nginx как $remote_addr).
// X-Forwarded-For клиент может подделать, поэтому для лимита не используем.
func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func failureKey(username, ip string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "\x00" + strings.TrimSpace(ip)
}

func (l *loginLimiter) allow(ip string) bool {
	if l == nil || ip == "" {
		return true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gcLocked(now)

	b := l.attempts[ip]
	if b == nil {
		return true
	}
	if now.Before(b.lockedUntil) {
		return false
	}
	if now.Sub(b.windowAt) > l.window {
		delete(l.attempts, ip)
		return true
	}
	return b.fails < l.maxFails
}

func (l *loginLimiter) recordFailure(ip, username string) {
	if l == nil {
		return
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if ip != "" {
		b := l.attempts[ip]
		if b == nil || now.Sub(b.windowAt) > l.window {
			l.attempts[ip] = &loginBucket{fails: 1, windowAt: now}
		} else {
			b.fails++
			if b.fails >= l.maxFails {
				b.lockedUntil = now.Add(l.lockout)
			}
		}
	}

	l.recordAuditLocked(now, ip, username)
}

func (l *loginLimiter) recordAuditLocked(now time.Time, ip, username string) {
	username = strings.TrimSpace(username)
	ip = strings.TrimSpace(ip)
	if username == "" && ip == "" {
		return
	}
	if username == "" {
		username = "(unknown)"
	}
	key := failureKey(username, ip)
	ev := l.failures[key]
	locked := false
	var lockedUntil time.Time
	if b := l.attempts[ip]; b != nil && now.Before(b.lockedUntil) {
		locked = true
		lockedUntil = b.lockedUntil
	}
	if ev == nil {
		l.failures[key] = &FailedLoginEvent{
			Username:    username,
			IP:          ip,
			Count:       1,
			FirstAt:     now,
			LastAt:      now,
			Locked:      locked,
			LockedUntil: lockedUntil,
		}
	} else {
		ev.Count++
		ev.LastAt = now
		ev.Locked = locked
		ev.LockedUntil = lockedUntil
	}
	l.trimAuditLocked(now)
}

func (l *loginLimiter) recordSuccess(ip string) {
	if l == nil || ip == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

func (l *loginLimiter) snapshotFailures() []FailedLoginEvent {
	if l == nil {
		return nil
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.trimAuditLocked(now)

	out := make([]FailedLoginEvent, 0, len(l.failures))
	for _, ev := range l.failures {
		cp := *ev
		if b := l.attempts[ev.IP]; b != nil && now.Before(b.lockedUntil) {
			cp.Locked = true
			cp.LockedUntil = b.lockedUntil
		} else {
			cp.Locked = false
			cp.LockedUntil = time.Time{}
		}
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].LastAt.Equal(out[j].LastAt) {
			return out[i].LastAt.After(out[j].LastAt)
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Username < out[j].Username
	})
	return out
}

func (l *loginLimiter) trimAuditLocked(now time.Time) {
	for k, ev := range l.failures {
		if now.Sub(ev.LastAt) > l.retain {
			delete(l.failures, k)
		}
	}
	if len(l.failures) <= l.maxAudit {
		return
	}
	type pair struct {
		key string
		at  time.Time
	}
	list := make([]pair, 0, len(l.failures))
	for k, ev := range l.failures {
		list = append(list, pair{key: k, at: ev.LastAt})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].at.Before(list[j].at) })
	drop := len(list) - l.maxAudit
	for i := 0; i < drop; i++ {
		delete(l.failures, list[i].key)
	}
}

func (l *loginLimiter) gcLocked(now time.Time) {
	if len(l.attempts) < 1024 {
		return
	}
	for ip, b := range l.attempts {
		if now.After(b.lockedUntil) && now.Sub(b.windowAt) > l.window {
			delete(l.attempts, ip)
		}
	}
}
