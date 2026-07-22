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
	tok, _, err := mgr.Issue("admin", auth.RoleAdministrator)
	if err != nil {
		t.Fatal(err)
	}

	h := csrfMW("bearer-secret", false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	tok, _, err := mgr.Issue("admin", auth.RoleAdministrator)
	if err != nil {
		t.Fatal(err)
	}
	csrf := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

	h := csrfMW("bearer-secret", false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	h := csrfMW("bearer-secret", false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	h := csrfMW("bearer-secret", false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
