package authmw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"geoatlas/internal/auth"
)

type stubUsers struct {
	role      string
	version   int64
	mustReset bool
	missing   bool
}

func (s stubUsers) Get(string) (auth.UserPublic, bool) {
	if s.missing {
		return auth.UserPublic{}, false
	}
	return auth.UserPublic{Username: "u", Role: s.role}, true
}
func (s stubUsers) SessionVersion(string) (int64, bool) {
	if s.missing {
		return 0, false
	}
	return s.version, true
}
func (s stubUsers) MustReset(string) bool { return s.mustReset }

type stubTokens struct {
	scope string
	ok    bool
}

func (s stubTokens) Verify(string) (string, bool) { return s.scope, s.ok }

func okHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func TestRequireLoginAuthDisabled(t *testing.T) {
	h := RequireLogin(BearerAuth{}, nil, nil, true)(okHandler(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequireLoginUnauthorized(t *testing.T) {
	h := RequireLogin(BearerAuth{}, nil, nil, false)(okHandler(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequireLoginBearer(t *testing.T) {
	ba := NewBearerAuth([]string{"secret-token"}, nil, nil)
	h := RequireLogin(ba, nil, nil, false)(okHandler(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRequireLoginSession(t *testing.T) {
	users := stubUsers{role: auth.RoleOperator, version: 1}
	load := func(*http.Request) (auth.Session, error) {
		return auth.Session{Username: "u", Role: auth.RoleOperator, SessionVersion: 1}, nil
	}
	h := RequireLogin(BearerAuth{}, load, users, false)(okHandler(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequireLoginMustReset(t *testing.T) {
	users := stubUsers{role: auth.RoleOperator, version: 1, mustReset: true}
	load := func(*http.Request) (auth.Session, error) {
		return auth.Session{Username: "u", Role: auth.RoleOperator, SessionVersion: 1}, nil
	}
	h := RequireLogin(BearerAuth{}, load, users, false)(okHandler(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequireAdminForbiddenForOperator(t *testing.T) {
	users := stubUsers{role: auth.RoleOperator, version: 1}
	load := func(*http.Request) (auth.Session, error) {
		return auth.Session{Username: "u", Role: auth.RoleOperator, SessionVersion: 1}, nil
	}
	h := RequireAdmin(BearerAuth{}, load, users, false)(okHandler(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequireAdminOK(t *testing.T) {
	users := stubUsers{role: auth.RoleAdministrator, version: 2}
	load := func(*http.Request) (auth.Session, error) {
		return auth.Session{Username: "admin", Role: auth.RoleAdministrator, SessionVersion: 2}, nil
	}
	h := RequireAdmin(BearerAuth{}, load, users, false)(okHandler(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRequireOpsNamedToken(t *testing.T) {
	ba := NewBearerAuth(nil, nil, stubTokens{scope: auth.ScopeOps, ok: true})
	h := RequireOps(ba, nil, nil, false, false)(okHandler(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer named")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestBearerScopeEnvIsAdmin(t *testing.T) {
	ba := NewBearerAuth([]string{"env"}, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer env")
	if !ba.OK(req, auth.ScopeAdmin) {
		t.Fatal("env bearer should be admin")
	}
	if !ba.Any(req) {
		t.Fatal("Any")
	}
}

func TestBearerScopeEnvOps(t *testing.T) {
	ba := NewBearerAuth([]string{"admin-tok"}, []string{"ops-tok-16chars!"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ops-tok-16chars!")
	if !ba.OK(req, auth.ScopeOps) {
		t.Fatal("ops env bearer should pass ops")
	}
	if ba.OK(req, auth.ScopeAdmin) {
		t.Fatal("ops env bearer must not be admin")
	}
	scope, ok := ba.Scope(req)
	if !ok || scope != auth.ScopeOps {
		t.Fatalf("scope=%q ok=%v", scope, ok)
	}
}

func TestSessionFromContext(t *testing.T) {
	_, ok := SessionFromContext(context.TODO())
	if ok {
		t.Fatal("empty ctx")
	}
	ctx := withSession(httptest.NewRequest(http.MethodGet, "/", nil).Context(), auth.Session{Username: "x"})
	s, ok := SessionFromContext(ctx)
	if !ok || s.Username != "x" {
		t.Fatalf("got %+v ok=%v", s, ok)
	}
}
