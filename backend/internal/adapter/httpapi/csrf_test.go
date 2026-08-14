package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"network_monitor/internal/auth"
)

func TestCSRFMiddlewareRejectsMissingToken(t *testing.T) {
	mgr, err := auth.NewSessionManager("csrf-test-secret-key-32bytes!!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := mgr.Issue("admin", auth.RoleAdministrator, 0)
	if err != nil {
		t.Fatal(err)
	}

	h := csrfMW(newBearerAuth([]string{"bearer-secret"}, nil), false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/upload-logs", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "csrf") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestCSRFMiddlewareAcceptsMatchingToken(t *testing.T) {
	mgr, err := auth.NewSessionManager("csrf-test-secret-key-32bytes!!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := mgr.Issue("admin", auth.RoleAdministrator, 0)
	if err != nil {
		t.Fatal(err)
	}
	csrf := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

	h := csrfMW(newBearerAuth([]string{"bearer-secret"}, nil), false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/upload-logs", nil)
	req.Host = "localhost"
	req.Header.Set("Origin", "http://localhost")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrf})
	req.Header.Set(auth.CSRFHeaderName, csrf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204 body=%s", rec.Code, rec.Body.String())
	}
}

func TestCSRFMiddlewareSkipsBearer(t *testing.T) {
	h := csrfMW(newBearerAuth([]string{"bearer-secret"}, nil), false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/ingest", nil)
	req.Header.Set("Authorization", "Bearer bearer-secret")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "whatever"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rec.Code)
	}
}

func TestCSRFMiddlewareRejectsBadOrigin(t *testing.T) {
	csrf := "token-value-32chars-minimum-here!!"
	h := csrfMW(newBearerAuth([]string{"bearer-secret"}, nil), false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodPost, "/upload-logs", nil)
	req.Host = "localhost"
	req.Header.Set("Origin", "https://evil.example")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "sess"})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrf})
	req.Header.Set(auth.CSRFHeaderName, csrf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403", rec.Code)
	}
}

func TestCSRFHostsEqualPortStrippedByProxy(t *testing.T) {
	cases := []struct {
		originHost, reqHost string
		want                bool
	}{
		{"155.212.245.143:8080", "155.212.245.143", true}, // nginx $host
		{"155.212.245.143:8080", "155.212.245.143:8080", true},
		{"localhost:8080", "localhost", true},
		{"localhost", "localhost:8080", true},
		{"example.com:443", "example.com", true},
		{"155.212.245.143:8080", "155.212.245.143:9090", false},
		{"evil.example:8080", "localhost:8080", false},
		{"evil.example", "localhost", false},
	}
	for _, tc := range cases {
		got := csrfHostsEqual(tc.originHost, tc.reqHost)
		if got != tc.want {
			t.Fatalf("csrfHostsEqual(%q, %q)=%v want %v", tc.originHost, tc.reqHost, got, tc.want)
		}
	}
}

func TestCSRFMiddlewareAcceptsOriginWithPortVsHostWithout(t *testing.T) {
	mgr, err := auth.NewSessionManager("csrf-test-secret-key-32bytes!!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := mgr.Issue("admin", auth.RoleAdministrator, 0)
	if err != nil {
		t.Fatal(err)
	}
	csrf := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

	h := csrfMW(newBearerAuth([]string{"bearer-secret"}, nil), false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", nil)
	req.Host = "155.212.245.143" // как после nginx $host
	req.Header.Set("Origin", "http://155.212.245.143:8080")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrf})
	req.Header.Set(auth.CSRFHeaderName, csrf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204 body=%s", rec.Code, rec.Body.String())
	}
}

func TestCSRFMiddlewareAcceptsIPOriginWhenHostIsUpstream(t *testing.T) {
	mgr, err := auth.NewSessionManager("csrf-test-secret-key-32bytes!!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := mgr.Issue("admin", auth.RoleAdministrator, 0)
	if err != nil {
		t.Fatal(err)
	}
	csrf := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

	h := csrfMW(newBearerAuth([]string{"bearer-secret"}, nil), false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	// Типичный сбой прокси: Host=backend:8080, Origin=публичный IP:8080
	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", nil)
	req.Host = "backend:8080"
	req.Header.Set("Origin", "http://155.212.245.143:8080")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrf})
	req.Header.Set(auth.CSRFHeaderName, csrf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204 body=%s", rec.Code, rec.Body.String())
	}
}

func TestCSRFMiddlewareAcceptsViaXForwardedHost(t *testing.T) {
	mgr, err := auth.NewSessionManager("csrf-test-secret-key-32bytes!!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := mgr.Issue("admin", auth.RoleAdministrator, 0)
	if err != nil {
		t.Fatal(err)
	}
	csrf := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

	h := csrfMW(newBearerAuth([]string{"bearer-secret"}, nil), false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", nil)
	req.Host = "backend:8080"
	req.Header.Set("X-Forwarded-Host", "app.example.com:8080")
	req.Header.Set("Origin", "http://app.example.com:8080")
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: csrf})
	req.Header.Set(auth.CSRFHeaderName, csrf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204 body=%s", rec.Code, rec.Body.String())
	}
}
