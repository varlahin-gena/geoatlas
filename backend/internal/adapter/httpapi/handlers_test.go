package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"network_monitor/internal/auth"
	"network_monitor/internal/config"
)

func TestUsersListDisabledWhenAuthModuleOff(t *testing.T) {
	h := &UsersHandler{Deps: &Deps{cfg: config.Config{AuthDisabled: true}}}
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body["error"] != "auth module disabled" {
		t.Fatalf("error = %v", body["error"])
	}
}

func TestHealthWithoutClickHouse(t *testing.T) {
	h := &HealthHandler{Deps: &Deps{
		cfg: config.Config{QueryTimeout: time.Minute},
		// systemUC nil → unavailable
	}}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	h.Health(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body["ok"] != false {
		t.Fatalf("ok = %v", body["ok"])
	}
	if body["status"] != "unavailable" {
		t.Fatalf("status = %v", body["status"])
	}
}

func TestAuthCheckAllowsBearer(t *testing.T) {
	h := &AuthHandler{Deps: &Deps{cfg: config.Config{APIAuthToken: "secret"}}}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/check", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h.Check(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAuthCheckAdminAllowsBearer(t *testing.T) {
	h := &AuthHandler{Deps: &Deps{cfg: config.Config{APIAuthToken: "secret"}}}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/check-admin", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h.CheckAdmin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestAuthCheckAdminOpenWhenAPIAuthDisabled(t *testing.T) {
	h := &AuthHandler{Deps: &Deps{cfg: config.Config{APIAuthDisabled: true}}}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/check-admin", nil)
	rec := httptest.NewRecorder()
	h.CheckAdmin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestRequireLoginMWAllowsBearer(t *testing.T) {
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), requireLoginMW(newBearerAuth([]string{"secret"}, nil), nil, nil, false))
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestRequireLoginMWUnauthorized(t *testing.T) {
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), requireLoginMW(newBearerAuth([]string{"secret"}, nil), nil, nil, false))
	req := httptest.NewRequest(http.MethodPost, "/api/ingest", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireLoginMWAllowsPreviousBearer(t *testing.T) {
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), requireLoginMW(newBearerAuth([]string{"new-token", "old-token"}, nil), nil, nil, false))
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.Header.Set("Authorization", "Bearer old-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 for previous token", rec.Code)
	}
}

func TestRequireAdminMWForbiddenForOperator(t *testing.T) {
	mgr, err := auth.NewSessionManager("mw-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := mgr.Issue("op", auth.RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), requireAdminMW(newBearerAuth([]string{"api-token"}, nil), mgr, nil, false))
	req := httptest.NewRequest(http.MethodGet, "/api/system/stats", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestRequireAdminMWAllowsAdministrator(t *testing.T) {
	mgr, err := auth.NewSessionManager("mw-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := mgr.Issue("admin", auth.RoleAdministrator)
	if err != nil {
		t.Fatal(err)
	}
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), requireAdminMW(newBearerAuth([]string{"api-token"}, nil), mgr, nil, false))
	req := httptest.NewRequest(http.MethodGet, "/api/system/stats", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestRequireAdminMWDeniesStickyAdminAfterDemote(t *testing.T) {
	mgr, err := auth.NewSessionManager("mw-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatal(err)
	}
	store, err := auth.NewUserStore(
		auth.User{Username: "keeper", PasswordHash: string(hash), Role: auth.RoleAdministrator},
		auth.User{Username: "was-admin", PasswordHash: string(hash), Role: auth.RoleAdministrator},
	)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := mgr.Issue("was-admin", auth.RoleAdministrator)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetRole("was-admin", auth.RoleOperator); err != nil {
		t.Fatal(err)
	}

	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), requireAdminMW(newBearerAuth([]string{"api-token"}, nil), mgr, store, false))
	req := httptest.NewRequest(http.MethodGet, "/api/system/stats", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 after demote", rec.Code)
	}
}

func TestRequireLoginMWAllowsSession(t *testing.T) {
	mgr, err := auth.NewSessionManager("mw-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := mgr.Issue("op", auth.RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), requireLoginMW(newBearerAuth([]string{"api-token"}, nil), mgr, nil, false))
	req := httptest.NewRequest(http.MethodPost, "/upload-logs", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestOpsMWForbidsOperator(t *testing.T) {
	mgr, err := auth.NewSessionManager("mw-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := auth.HashPassword("password1")
	if err != nil {
		t.Fatal(err)
	}
	store, err := auth.NewUserStore(
		auth.User{Username: "admin", PasswordHash: string(hash), Role: auth.RoleAdministrator},
		auth.User{Username: "op", PasswordHash: string(hash), Role: auth.RoleOperator},
	)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := mgr.Issue("op", auth.RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), requireOpsMW(newBearerAuth([]string{"api-token"}, nil), mgr, store, false, false))
	req := httptest.NewRequest(http.MethodPost, "/upload-logs", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestOpsMWAllowsBearer(t *testing.T) {
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), requireOpsMW(newBearerAuth([]string{"secret"}, nil), nil, nil, false, false))
	req := httptest.NewRequest(http.MethodGet, "/api/ingest/stats", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestOpsMWAPIAuthDisabledAllows(t *testing.T) {
	// API_AUTH_DISABLED — мутирующие эндпоинты открыты.
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), requireOpsMW(newBearerAuth(nil, nil), nil, nil, true, false))
	req := httptest.NewRequest(http.MethodPost, "/api/ingest", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestParseOptionalLimitDefaults(t *testing.T) {
	if got := parseOptionalLimit(""); got != defaultEventsLimit {
		t.Fatalf("empty = %d, want %d", got, defaultEventsLimit)
	}
	if got := parseOptionalLimit("0"); got != defaultEventsLimit {
		t.Fatalf("zero = %d, want %d", got, defaultEventsLimit)
	}
	if got := parseOptionalLimit("-1"); got != defaultEventsLimit {
		t.Fatalf("neg = %d, want %d", got, defaultEventsLimit)
	}
	if got := parseOptionalLimit("500"); got != 500 {
		t.Fatalf("500 = %d", got)
	}
	if got := parseOptionalLimit("999999"); got != maxEventsLimit {
		t.Fatalf("clamp = %d, want %d", got, maxEventsLimit)
	}
}

func TestRouteLabelUsesMuxTemplate(t *testing.T) {
	r := mux.NewRouter()
	var got string
	r.HandleFunc("/api/events", func(w http.ResponseWriter, req *http.Request) {
		got = routeLabel(req)
		w.WriteHeader(http.StatusNoContent)
	}).Methods("GET")

	req := httptest.NewRequest(http.MethodGet, "/api/events?days=1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if got != "/api/events" {
		t.Fatalf("routeLabel = %q, want /api/events", got)
	}
}
