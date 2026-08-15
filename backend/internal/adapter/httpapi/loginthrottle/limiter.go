// Package loginthrottle — per-IP login lockout + failed-login audit + trusted-proxy client IP.
package loginthrottle

import (
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Limiter — per-IP throttle + аудит неуспешных попыток по паре username+IP.
type Limiter struct {
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
	Username    string    `json:"username"`
	IP          string    `json:"ip"`
	Count       int       `json:"count"`
	FirstAt     time.Time `json:"first_at"`
	LastAt      time.Time `json:"last_at"`
	Locked      bool      `json:"locked"`
	LockedUntil time.Time `json:"locked_until,omitempty"`
}

// New — limiter with defaults when params are non-positive.
func New(maxFails int, window, lockout time.Duration) *Limiter {
	if maxFails <= 0 {
		maxFails = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	if lockout <= 0 {
		lockout = 5 * time.Minute
	}
	return &Limiter{
		attempts: make(map[string]*loginBucket),
		failures: make(map[string]*FailedLoginEvent),
		maxFails: maxFails,
		window:   window,
		lockout:  lockout,
		retain:   24 * time.Hour,
		maxAudit: 200,
	}
}

var (
	trustedProxyMu   sync.RWMutex
	trustedProxyNets []*net.IPNet
	trustedProxyHost map[string]struct{}
)

// ConfigureTrustedProxies — CIDR и/или hostnames (frontend). Loopback всегда доверен.
func ConfigureTrustedProxies(entries []string) {
	nets := make([]*net.IPNet, 0, len(entries)+2)
	hosts := make(map[string]struct{})
	addCIDR := func(cidr string) {
		if _, n, err := net.ParseCIDR(cidr); err == nil {
			nets = append(nets, n)
		}
	}
	addCIDR("127.0.0.0/8")
	addCIDR("::1/128")

	for _, raw := range entries {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.Contains(raw, "/") {
			addCIDR(raw)
			continue
		}
		if ip := net.ParseIP(raw); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		hosts[strings.ToLower(raw)] = struct{}{}
	}

	trustedProxyMu.Lock()
	trustedProxyNets = nets
	trustedProxyHost = hosts
	trustedProxyMu.Unlock()
}

func init() {
	ConfigureTrustedProxies([]string{"frontend"})
}

func remoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isTrustedProxy(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	trustedProxyMu.RLock()
	nets := trustedProxyNets
	hosts := trustedProxyHost
	trustedProxyMu.RUnlock()

	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	for name := range hosts {
		addrs, err := net.LookupHost(name)
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if parsed := net.ParseIP(a); parsed != nil && parsed.Equal(ip) {
				return true
			}
		}
	}
	return false
}

// ClientIP — IP для throttle/аудита.
// X-Real-IP только если RemoteAddr — trusted proxy (nginx/frontend).
// X-Forwarded-For клиент может подделать — не используем.
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	remote := remoteIP(r)
	if isTrustedProxy(remote) {
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			if ip := net.ParseIP(xri); ip != nil {
				return ip.String()
			}
		}
	}
	return remote
}

func failureKey(username, ip string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "\x00" + strings.TrimSpace(ip)
}

func (l *Limiter) Allow(ip string) bool {
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

func (l *Limiter) RecordFailure(ip, username string) {
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

func (l *Limiter) recordAuditLocked(now time.Time, ip, username string) {
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

func (l *Limiter) RecordSuccess(ip string) {
	if l == nil || ip == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

func (l *Limiter) SnapshotFailures() []FailedLoginEvent {
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

func (l *Limiter) trimAuditLocked(now time.Time) {
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

func (l *Limiter) gcLocked(now time.Time) {
	if len(l.attempts) < 1024 {
		return
	}
	for ip, b := range l.attempts {
		if now.After(b.lockedUntil) && now.Sub(b.windowAt) > l.window {
			delete(l.attempts, ip)
		}
	}
}
