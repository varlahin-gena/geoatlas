package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"geoatlas/internal/auth"
	"geoatlas/internal/config"
)

func TestCheckOpsAllowsOpsBearer(t *testing.T) {
	dir := t.TempDir()
	store, err := auth.OpenOrCreateTokenStore(filepath.Join(dir, "tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.Create("ops", auth.ScopeOps, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	h := &AuthHandler{AuthDeps: &AuthDeps{
		cfg:       config.Config{APIAuthToken: "env-admin-token"},
		apiTokens: store,
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/check-ops", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	h.CheckOps(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("check-ops status=%d", rec.Code)
	}

	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/auth/check-admin", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+secret)
	rec = httptest.NewRecorder()
	h.CheckAdmin(rec, reqAdmin)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("check-admin with ops token: %d want 401", rec.Code)
	}
}

func TestScopedBearerOpsVsAdmin(t *testing.T) {
	dir := t.TempDir()
	store, err := auth.OpenOrCreateTokenStore(filepath.Join(dir, "tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, opsSecret, err := store.Create("ops-bot", auth.ScopeOps, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	ba := newBearerAuth(nil, nil, store)

	opsOK := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), requireOpsMW(ba, nil, nil, false, false))
	adminOK := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), requireAdminMW(ba, nil, nil, false))

	reqOps := httptest.NewRequest(http.MethodGet, "/api/ingest/stats", nil)
	reqOps.Header.Set("Authorization", "Bearer "+opsSecret)
	rec := httptest.NewRecorder()
	opsOK.ServeHTTP(rec, reqOps)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("ops token on opsMW: %d", rec.Code)
	}

	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+opsSecret)
	rec = httptest.NewRecorder()
	adminOK.ServeHTTP(rec, reqAdmin)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ops token on adminMW: %d want 401", rec.Code)
	}
}
