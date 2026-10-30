package authmw

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"geoatlas/internal/auth"
)

func TestRequireOpsPaths(t *testing.T) {
	ok := okHandler(t)

	h := RequireOps(BearerAuth{}, nil, nil, true, false)(ok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("apiAuthDisabled: %d", rec.Code)
	}

	ba := NewBearerAuth(nil, []string{"ops-secret"}, stubTokens{scope: auth.ScopeOps, ok: true})
	h = RequireOps(ba, nil, nil, false, false)(ok)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ops-secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("bearer ops: %d", rec.Code)
	}

	h = RequireOps(BearerAuth{}, nil, nil, false, false)(ok)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("nil load: %d", rec.Code)
	}

	loadErr := func(*http.Request) (auth.Session, error) { return auth.Session{}, errors.New("no") }
	h = RequireOps(BearerAuth{}, loadErr, nil, false, false)(ok)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("load err: %d", rec.Code)
	}

	users := stubUsers{role: auth.RoleOperator, version: 1}
	loadOp := func(*http.Request) (auth.Session, error) {
		return auth.Session{Username: "u", Role: auth.RoleOperator, SessionVersion: 1}, nil
	}
	h = RequireOps(BearerAuth{}, loadOp, users, false, false)(ok)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("ops forbidden: %d", rec.Code)
	}

	admin := stubUsers{role: auth.RoleAdministrator, version: 1}
	loadAdm := func(*http.Request) (auth.Session, error) {
		return auth.Session{Username: "a", Role: auth.RoleAdministrator, SessionVersion: 1}, nil
	}
	h = RequireOps(BearerAuth{}, loadAdm, admin, false, false)(ok)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("admin ops: %d", rec.Code)
	}

	reset := stubUsers{role: auth.RoleAdministrator, version: 1, mustReset: true}
	h = RequireOps(BearerAuth{}, loadAdm, reset, false, false)(ok)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("must reset: %d", rec.Code)
	}

	missing := stubUsers{missing: true}
	h = RequireOps(BearerAuth{}, loadAdm, missing, false, false)(ok)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("live miss: %d", rec.Code)
	}

	//nolint:staticcheck // SA1012: explicit nil-context path for SessionFromContext
	nilCtx := (context.Context)(nil)
	_, okSess := SessionFromContext(nilCtx)
	if okSess {
		t.Fatal("nil ctx")
	}
	ctx := ContextWithSession(context.Background(), auth.Session{Username: "x"})
	if s, ok := SessionFromContext(ctx); !ok || s.Username != "x" {
		t.Fatalf("%+v %v", s, ok)
	}
}

func TestRequireAdminMorePaths(t *testing.T) {
	ok := okHandler(t)
	h := RequireAdmin(BearerAuth{}, nil, nil, true)(ok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatal(rec.Code)
	}

	ba := NewBearerAuth([]string{"admin-secret"}, nil, nil)
	h = RequireAdmin(ba, nil, nil, false)(ok)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer admin-secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatal(rec.Code)
	}

	h = RequireAdmin(BearerAuth{}, nil, nil, false)(ok)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatal(rec.Code)
	}

	loadErr := func(*http.Request) (auth.Session, error) { return auth.Session{}, errors.New("x") }
	h = RequireAdmin(BearerAuth{}, loadErr, nil, false)(ok)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatal(rec.Code)
	}

	reset := stubUsers{role: auth.RoleAdministrator, version: 1, mustReset: true}
	load := func(*http.Request) (auth.Session, error) {
		return auth.Session{Username: "a", Role: auth.RoleAdministrator, SessionVersion: 1}, nil
	}
	h = RequireAdmin(BearerAuth{}, load, reset, false)(ok)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatal(rec.Code)
	}
}
