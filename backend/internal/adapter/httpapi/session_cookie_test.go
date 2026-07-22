package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"network_monitor/internal/auth"
)

func TestSessionCookieRoundTrip(t *testing.T) {
	mgr, err := auth.NewSessionManager("cookie-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := mgr.Issue("operator", auth.RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	SetCookie(rec, req, token, mgr.TTL())
	cookies := rec.Result().Cookies()
	var sessCookie, csrfCookie *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case auth.CookieName:
			sessCookie = c
		case auth.CSRFCookieName:
			csrfCookie = c
		}
	}
	if sessCookie == nil {
		t.Fatalf("missing session cookie in %+v", cookies)
	}
	if csrfCookie == nil || csrfCookie.Value == "" {
		t.Fatalf("missing csrf cookie in %+v", cookies)
	}
	if csrfCookie.HttpOnly {
		t.Fatal("csrf cookie must be readable by JS (HttpOnly=false)")
	}
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(sessCookie)
	sess, err := SessionFromRequest(req2, mgr)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Role != auth.RoleOperator {
		t.Fatalf("role = %s", sess.Role)
	}
	if sessCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("SameSite = %v, want Strict", sessCookie.SameSite)
	}
}
