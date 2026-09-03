package loginthrottle

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoginLimiterLockout(t *testing.T) {
	lim := New(3, time.Minute, time.Minute)
	ip := "203.0.113.10"
	if !lim.Allow(ip, "admin") {
		t.Fatal("should allow initially")
	}
	for i := 0; i < 3; i++ {
		lim.RecordFailure(ip, "admin")
	}
	if lim.Allow(ip, "admin") {
		t.Fatal("should be locked after max fails")
	}
	lim.RecordSuccess(ip, "admin")
	if !lim.Allow(ip, "admin") {
		t.Fatal("success should clear lockout")
	}
}

func TestLoginLimiterAccountLockoutAcrossIPs(t *testing.T) {
	lim := New(3, time.Minute, time.Minute)
	for _, ip := range []string{"203.0.113.10", "203.0.113.11", "203.0.113.12"} {
		lim.RecordFailure(ip, "admin")
	}
	// Same account from a fresh IP must still be locked.
	if lim.Allow("198.51.100.50", "admin") {
		t.Fatal("account should be locked across IPs")
	}
	// Other accounts from a used IP remain allowed (IP bucket not exhausted alone).
	if !lim.Allow("203.0.113.10", "operator") {
		t.Fatal("other username should still be allowed")
	}
	lim.RecordSuccess("198.51.100.50", "admin")
	if !lim.Allow("198.51.100.99", "admin") {
		t.Fatal("success should clear account lockout")
	}
}

func TestClientIPPrefersXRealIP(t *testing.T) {
	ConfigureTrustedProxies([]string{"10.0.0.1"})
	t.Cleanup(func() { ConfigureTrustedProxies([]string{"frontend"}) })

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Real-IP", "198.51.100.7")
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	if got := ClientIP(req); got != "198.51.100.7" {
		t.Fatalf("got %q, want X-Real-IP", got)
	}
}

func TestClientIPIgnoresXRealIPFromUntrusted(t *testing.T) {
	ConfigureTrustedProxies([]string{"frontend"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "203.0.113.50:9999"
	req.Header.Set("X-Real-IP", "198.51.100.7")
	if got := ClientIP(req); got != "203.0.113.50" {
		t.Fatalf("got %q, want RemoteAddr (untrusted must not spoof via X-Real-IP)", got)
	}
}

func TestClientIPIgnoresClientXFF(t *testing.T) {
	ConfigureTrustedProxies([]string{"10.0.0.1"})
	t.Cleanup(func() { ConfigureTrustedProxies([]string{"frontend"}) })

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.1")
	if got := ClientIP(req); got != "10.0.0.1" {
		t.Fatalf("got %q, want RemoteAddr host (XFF must not spoof throttle)", got)
	}
}

func TestClientIPIgnoresInvalidXRealIP(t *testing.T) {
	ConfigureTrustedProxies([]string{"10.0.0.1"})
	t.Cleanup(func() { ConfigureTrustedProxies([]string{"frontend"}) })

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Real-IP", "not-an-ip")
	if got := ClientIP(req); got != "10.0.0.1" {
		t.Fatalf("got %q, want RemoteAddr fallback", got)
	}
}

func TestRequestFromTrustedHop(t *testing.T) {
	ConfigureTrustedProxies([]string{"10.0.0.1"})
	t.Cleanup(func() { ConfigureTrustedProxies([]string{"frontend"}) })

	ok := httptest.NewRequest(http.MethodGet, "/api/live", nil)
	ok.RemoteAddr = "10.0.0.1:80"
	if !RequestFromTrustedHop(ok) {
		t.Fatal("trusted hop should pass")
	}
	loop := httptest.NewRequest(http.MethodGet, "/live", nil)
	loop.RemoteAddr = "127.0.0.1:1"
	if !RequestFromTrustedHop(loop) {
		t.Fatal("loopback should pass")
	}
	bad := httptest.NewRequest(http.MethodGet, "/api/live", nil)
	bad.RemoteAddr = "203.0.113.9:443"
	if RequestFromTrustedHop(bad) {
		t.Fatal("external hop should fail")
	}
}

func TestLoginThrottleWindowReset(t *testing.T) {
	lim := New(2, 20*time.Millisecond, 30*time.Millisecond)
	ip := "198.51.100.1"
	lim.RecordFailure(ip, "op")
	lim.RecordFailure(ip, "op")
	if lim.Allow(ip, "op") {
		t.Fatal("expected lockout")
	}
	time.Sleep(40 * time.Millisecond)
	if !lim.Allow(ip, "op") {
		t.Fatal("expected unlock after lockout")
	}
}

func TestFailedLoginSnapshotAggregatesByUserAndIP(t *testing.T) {
	lim := New(10, time.Minute, time.Minute)
	lim.RecordFailure("10.0.0.1", "Admin")
	lim.RecordFailure("10.0.0.1", "admin")
	lim.RecordFailure("10.0.0.2", "admin")
	lim.RecordFailure("10.0.0.1", "operator")

	snap := lim.SnapshotFailures()
	if len(snap) != 3 {
		t.Fatalf("len=%d want 3: %+v", len(snap), snap)
	}
	var adminLocal *FailedLoginEvent
	for i := range snap {
		if strings.EqualFold(snap[i].Username, "admin") && snap[i].IP == "10.0.0.1" {
			adminLocal = &snap[i]
		}
	}
	if adminLocal == nil || adminLocal.Count != 2 {
		t.Fatalf("admin@10.0.0.1 = %+v", adminLocal)
	}
	if snap[0].LastAt.Before(snap[len(snap)-1].LastAt) {
		t.Fatal("expected newest first")
	}
}
