package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"geoatlas/internal/auth"
	"geoatlas/internal/config"
	usecaseauth "geoatlas/internal/usecase/auth"
)

func TestReauthCheckerRequiresPasswordForCookieSession(t *testing.T) {
	mgr, err := auth.NewSessionManager("reauth-test-secret-key-32bytes!!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("admin-pass-1234")
	if err != nil {
		t.Fatal(err)
	}
	users, err := auth.NewUserStore(auth.User{
		Username: "admin", PasswordHash: string(hash), Role: auth.RoleAdministrator,
	})
	if err != nil {
		t.Fatal(err)
	}
	authUC := usecaseauth.New(users, mgr)
	checker := NewReauthChecker(config.Config{}, authUC, mgr, nil)

	sv, ok := users.SessionVersion("admin")
	if !ok {
		t.Fatal("admin session version missing")
	}
	tok, _, err := mgr.Issue("admin", auth.RoleAdministrator, sv)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout-all", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
	rec := httptest.NewRecorder()
	if _, ok := checker.Require(rec, req, ""); ok {
		t.Fatal("expected missing password to fail")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing password: status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec2 := httptest.NewRecorder()
	if _, ok := checker.Require(rec2, req, "wrong"); ok {
		t.Fatal("expected wrong password to fail")
	}
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: status=%d body=%s", rec2.Code, rec2.Body.String())
	}

	rec3 := httptest.NewRecorder()
	actor, ok := checker.Require(rec3, req, "admin-pass-1234")
	if !ok || actor != "admin" {
		t.Fatalf("valid password: ok=%v actor=%q status=%d body=%s", ok, actor, rec3.Code, rec3.Body.String())
	}
}

func TestReauthCheckerSkipsBearer(t *testing.T) {
	checker := NewReauthChecker(config.Config{APIAuthToken: "env-admin-token"}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/tokens/id", nil)
	req.Header.Set("Authorization", "Bearer env-admin-token")
	rec := httptest.NewRecorder()
	if _, ok := checker.Require(rec, req, ""); !ok {
		t.Fatalf("bearer should skip reauth, status=%d", rec.Code)
	}
}
