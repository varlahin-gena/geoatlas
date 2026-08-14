package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoginLimiterLockout(t *testing.T) {
	lim := newLoginLimiter(3, time.Minute, time.Minute)
	ip := "203.0.113.10"
	if !lim.allow(ip) {
		t.Fatal("should allow initially")
	}
	for i := 0; i < 3; i++ {
		lim.recordFailure(ip, "admin")
	}
	if lim.allow(ip) {
		t.Fatal("should be locked after max fails")
	}
	lim.recordSuccess(ip)
	if !lim.allow(ip) {
		t.Fatal("success should clear lockout")
	}
}

func TestClientIPPrefersXRealIP(t *testing.T) {
	ConfigureTrustedProxies([]string{"10.0.0.1"})
	t.Cleanup(func() { ConfigureTrustedProxies([]string{"frontend"}) })

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Real-IP", "198.51.100.7")
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	if got := clientIP(req); got != "198.51.100.7" {
		t.Fatalf("got %q, want X-Real-IP", got)
	}
}

func TestClientIPIgnoresXRealIPFromUntrusted(t *testing.T) {
	ConfigureTrustedProxies([]string{"frontend"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "203.0.113.50:9999"
	req.Header.Set("X-Real-IP", "198.51.100.7")
	if got := clientIP(req); got != "203.0.113.50" {
		t.Fatalf("got %q, want RemoteAddr (untrusted must not spoof via X-Real-IP)", got)
	}
}

func TestClientIPIgnoresClientXFF(t *testing.T) {
	ConfigureTrustedProxies([]string{"10.0.0.1"})
	t.Cleanup(func() { ConfigureTrustedProxies([]string{"frontend"}) })

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.1")
	if got := clientIP(req); got != "10.0.0.1" {
		t.Fatalf("got %q, want RemoteAddr host (XFF must not spoof throttle)", got)
	}
}

func TestClientIPIgnoresInvalidXRealIP(t *testing.T) {
	ConfigureTrustedProxies([]string{"10.0.0.1"})
	t.Cleanup(func() { ConfigureTrustedProxies([]string{"frontend"}) })

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Real-IP", "not-an-ip")
	if got := clientIP(req); got != "10.0.0.1" {
		t.Fatalf("got %q, want RemoteAddr fallback", got)
	}
}

func TestLoginThrottleWindowReset(t *testing.T) {
	lim := newLoginLimiter(2, 20*time.Millisecond, 30*time.Millisecond)
	ip := "198.51.100.1"
	lim.recordFailure(ip, "op")
	lim.recordFailure(ip, "op")
	if lim.allow(ip) {
		t.Fatal("expected lockout")
	}
	time.Sleep(40 * time.Millisecond)
	if !lim.allow(ip) {
		t.Fatal("expected unlock after lockout")
	}
}

func TestFailedLoginSnapshotAggregatesByUserAndIP(t *testing.T) {
	lim := newLoginLimiter(10, time.Minute, time.Minute)
	lim.recordFailure("10.0.0.1", "Admin")
	lim.recordFailure("10.0.0.1", "admin")
	lim.recordFailure("10.0.0.2", "admin")
	lim.recordFailure("10.0.0.1", "operator")

	snap := lim.snapshotFailures()
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
