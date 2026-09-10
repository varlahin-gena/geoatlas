// Package loginthrottle — per-IP и per-account login lockout + failed-login audit + trusted-proxy client IP.
package loginthrottle

import (
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Limiter — throttle по IP и по username + аудит неуспешных попыток по паре username+IP.
type Limiter struct {
	mu           sync.Mutex
	attempts     map[string]*loginBucket      // key: IP
	userAttempts map[string]*loginBucket      // key: lower(username)
	failures     map[string]*FailedLoginEvent // key: lower(username)|ip
	maxFails     int
	window       time.Duration
	lockout      time.Duration
	retain       time.Duration
	maxAudit     int
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
		attempts:     make(map[string]*loginBucket),
		userAttempts: make(map[string]*loginBucket),
		failures:     make(map[string]*FailedLoginEvent),
		maxFails:     maxFails,
		window:       window,
		lockout:      lockout,
		retain:       24 * time.Hour,
		maxAudit:     200,
	}
}

// trustedSet — неизменяемый снапшот доверенных прокси. Хостнеймы резолвятся
// заранее, чтобы путь обработки запроса не ходил в DNS.
type trustedSet struct {
	nets       []*net.IPNet
	hostNames  []string
	hostIPs    []net.IP
	resolvedAt time.Time
}

var (
	trusted              atomic.Pointer[trustedSet]
	trustedRefreshing    atomic.Bool
	trustedEnsureRunning atomic.Bool

	// Подменяются в тестах.
	lookupHost     = net.LookupHost
	trustedHostTTL = 30 * time.Second
	// Пока hostname (frontend) ещё не резолвится — backend уже up, nginx ещё нет —
	// короче дёргаем DNS, иначе GA_REQUIRE_PROXY=1 отдаёт 403 на логин после update.
	trustedEmptyRetry = 500 * time.Millisecond
	trustedEnsureWait = 2 * time.Minute
)

func loopbackNets() []*net.IPNet {
	nets := make([]*net.IPNet, 0, 2)
	for _, cidr := range []string{"127.0.0.0/8", "::1/128"} {
		if _, n, err := net.ParseCIDR(cidr); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}

// loadTrusted — снапшот; до ConfigureTrustedProxies доверяем только loopback.
func loadTrusted() *trustedSet {
	if tp := trusted.Load(); tp != nil {
		return tp
	}
	trusted.CompareAndSwap(nil, &trustedSet{nets: loopbackNets(), resolvedAt: time.Now()})
	return trusted.Load()
}

// ConfigureTrustedProxies — CIDR и/или hostnames (frontend). Loopback всегда доверен.
// Хостнеймы резолвятся здесь синхронно: вызывается один раз на старте, и первый
// же запрос должен корректно опознаваться как пришедший через прокси.
func ConfigureTrustedProxies(entries []string) {
	nets := loopbackNets()
	var hostNames []string
	seen := make(map[string]struct{})

	for _, raw := range entries {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.Contains(raw, "/") {
			if _, n, err := net.ParseCIDR(raw); err == nil {
				nets = append(nets, n)
			}
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
		name := strings.ToLower(raw)
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		hostNames = append(hostNames, name)
	}

	next := &trustedSet{nets: nets, hostNames: hostNames}
	resolveHosts(next, nil)
	trusted.Store(next)
	if len(hostNames) > 0 && len(next.hostIPs) == 0 {
		ensureTrustedHostsResolved()
	}
}

// resolveHosts заполняет hostIPs. prev — снапшот предыдущих адресов: если DNS
// недоступен, держим их, иначе временный сбой резолвера снимет доверие к прокси
// и весь API начнёт отвечать 403 при GA_REQUIRE_PROXY=1.
func resolveHosts(s *trustedSet, prev []net.IP) {
	s.resolvedAt = time.Now()
	if len(s.hostNames) == 0 {
		return
	}
	ips := make([]net.IP, 0, len(s.hostNames)*2)
	for _, name := range s.hostNames {
		addrs, err := lookupHost(name)
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ip := net.ParseIP(a); ip != nil {
				ips = append(ips, ip)
			}
		}
	}
	if len(ips) == 0 && len(prev) > 0 {
		ips = prev
	}
	s.hostIPs = ips
}

func refreshTrustedHosts() {
	cur := trusted.Load()
	if cur == nil || len(cur.hostNames) == 0 {
		return
	}
	next := &trustedSet{nets: cur.nets, hostNames: cur.hostNames}
	resolveHosts(next, cur.hostIPs)
	trusted.Store(next)
}

// ensureTrustedHostsResolved — cold-start race: backend healthy раньше frontend,
// LookupHost("frontend") пустой → proxy gate режет UI. Догоняем DNS в фоне.
func ensureTrustedHostsResolved() {
	if !trustedEnsureRunning.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer trustedEnsureRunning.Store(false)
		backoff := trustedEmptyRetry
		if backoff <= 0 {
			backoff = 500 * time.Millisecond
		}
		deadline := time.Now().Add(trustedEnsureWait)
		for time.Now().Before(deadline) {
			cur := trusted.Load()
			if cur == nil || len(cur.hostNames) == 0 || len(cur.hostIPs) > 0 {
				return
			}
			refreshTrustedHosts()
			if tp := trusted.Load(); tp != nil && len(tp.hostIPs) > 0 {
				return
			}
			time.Sleep(backoff)
			if backoff < 5*time.Second {
				backoff *= 2
			}
		}
	}()
}

func needsTrustedHostRefresh(tp *trustedSet) bool {
	if tp == nil || len(tp.hostNames) == 0 {
		return false
	}
	if len(tp.hostIPs) == 0 {
		return true
	}
	return time.Since(tp.resolvedAt) > trustedHostTTL
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
	tp := loadTrusted()
	for _, n := range tp.nets {
		if n.Contains(ip) {
			return true
		}
	}
	if len(tp.hostNames) == 0 {
		return false
	}
	// Снапшот устарел или hostname ещё не резолвился — одна фоновая горутина;
	// запрос не ждёт DNS и отвечает по текущему снапшоту.
	if needsTrustedHostRefresh(tp) && trustedRefreshing.CompareAndSwap(false, true) {
		go func() {
			defer trustedRefreshing.Store(false)
			refreshTrustedHosts()
		}()
	}
	for _, h := range tp.hostIPs {
		if h.Equal(ip) {
			return true
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

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// RequestFromTrustedHop — RemoteAddr loopback или GA_TRUSTED_PROXIES (nginx/frontend).
func RequestFromTrustedHop(r *http.Request) bool {
	return isTrustedProxy(remoteIP(r))
}

func bucketAllows(b *loginBucket, now time.Time, window time.Duration, maxFails int) (allow bool, stale bool) {
	if b == nil {
		return true, false
	}
	if now.Before(b.lockedUntil) {
		return false, false
	}
	if now.Sub(b.windowAt) > window {
		return true, true
	}
	return b.fails < maxFails, false
}

func bumpBucket(m map[string]*loginBucket, key string, now time.Time, window, lockout time.Duration, maxFails int) {
	if key == "" || m == nil {
		return
	}
	b := m[key]
	if b == nil || now.Sub(b.windowAt) > window {
		m[key] = &loginBucket{fails: 1, windowAt: now}
		return
	}
	b.fails++
	if b.fails >= maxFails {
		b.lockedUntil = now.Add(lockout)
	}
}

func lockState(b *loginBucket, now time.Time) (locked bool, until time.Time) {
	if b != nil && now.Before(b.lockedUntil) {
		return true, b.lockedUntil
	}
	return false, time.Time{}
}

// Allow — false, если IP или username в lockout (распределённый брутфорс по аккаунту).
func (l *Limiter) Allow(ip, username string) bool {
	if l == nil {
		return true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gcLocked(now)

	if ip != "" {
		allow, stale := bucketAllows(l.attempts[ip], now, l.window, l.maxFails)
		if stale {
			delete(l.attempts, ip)
		}
		if !allow {
			return false
		}
	}
	user := normalizeUsername(username)
	if user != "" {
		allow, stale := bucketAllows(l.userAttempts[user], now, l.window, l.maxFails)
		if stale {
			delete(l.userAttempts, user)
		}
		if !allow {
			return false
		}
	}
	return true
}

func (l *Limiter) RecordFailure(ip, username string) {
	if l == nil {
		return
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	bumpBucket(l.attempts, strings.TrimSpace(ip), now, l.window, l.lockout, l.maxFails)
	bumpBucket(l.userAttempts, normalizeUsername(username), now, l.window, l.lockout, l.maxFails)
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
	locked, lockedUntil := lockState(l.attempts[ip], now)
	if ulocked, uUntil := lockState(l.userAttempts[normalizeUsername(username)], now); ulocked {
		locked = true
		if uUntil.After(lockedUntil) {
			lockedUntil = uUntil
		}
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

func (l *Limiter) RecordSuccess(ip, username string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if ip != "" {
		delete(l.attempts, strings.TrimSpace(ip))
	}
	if user := normalizeUsername(username); user != "" {
		delete(l.userAttempts, user)
	}
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
		locked, until := lockState(l.attempts[ev.IP], now)
		if ulocked, uUntil := lockState(l.userAttempts[normalizeUsername(ev.Username)], now); ulocked {
			locked = true
			if uUntil.After(until) {
				until = uUntil
			}
		}
		cp.Locked = locked
		cp.LockedUntil = until
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
	gcMap := func(m map[string]*loginBucket, threshold int) {
		if len(m) < threshold {
			return
		}
		for k, b := range m {
			if now.After(b.lockedUntil) && now.Sub(b.windowAt) > l.window {
				delete(m, k)
			}
		}
	}
	gcMap(l.attempts, 1024)
	gcMap(l.userAttempts, 1024)
}
