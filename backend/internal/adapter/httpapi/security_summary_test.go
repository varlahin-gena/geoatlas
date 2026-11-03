package httpapi

import (
	"os"
	"path/filepath"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"geoatlas/internal/adapter/httpapi/loginthrottle"
	"geoatlas/internal/auth"
	usecaseaudit "geoatlas/internal/usecase/auditlog"
	usecasetls "geoatlas/internal/usecase/tls"
)

type postureUsers struct {
	items []auth.UserPublic
}

func (p postureUsers) Get(string) (auth.UserPublic, bool) { return auth.UserPublic{}, false }
func (p postureUsers) SessionVersion(string) (int64, bool) { return 0, false }
func (p postureUsers) MustReset(string) bool               { return false }
func (p postureUsers) List() []auth.UserPublic             { return p.items }

type postureTokens struct {
	items []auth.APITokenPublic
}

func (p postureTokens) List() []auth.APITokenPublic { return p.items }
func (p postureTokens) Create(string, string, time.Time) (auth.APITokenPublic, string, error) {
	return auth.APITokenPublic{}, "", nil
}
func (p postureTokens) Rotate(string) (auth.APITokenPublic, string, error) {
	return auth.APITokenPublic{}, "", nil
}
func (p postureTokens) Revoke(string) error          { return nil }
func (p postureTokens) Verify(string) (string, bool) { return "", false }

type countingAuditStore struct {
	fakeAuditStore
	counts []usecaseaudit.ActionCount
}

func (c *countingAuditStore) CountAuditByAction(_ context.Context, _ time.Time, _ string) ([]usecaseaudit.ActionCount, error) {
	return c.counts, nil
}

func TestGetSecuritySummary(t *testing.T) {
	reg, err := auth.NewSessionRegistry("")
	if err != nil {
		t.Fatal(err)
	}
	_ = reg.Register(auth.SessionRecord{ID: "s1", Username: "admin", Role: auth.RoleAdministrator, IP: "127.0.0.1"})
	lim := loginthrottle.New(3, time.Minute, time.Minute)
	lim.RecordFailure("9.9.9.9", "eve")
	logs := usecaseaudit.New(&countingAuditStore{
		fakeAuditStore: fakeAuditStore{
			audit: []usecaseaudit.AuditEvent{{
				Timestamp: time.Now().UTC(),
				Actor:     "admin",
				Action:    "auth.login.success",
				Result:    "succeeded",
			}},
		},
		counts: []usecaseaudit.ActionCount{
			{Action: "security.csrf.reject", Count: 2},
			{Action: "security.proxy_gate.reject", Count: 1},
			{Action: "security.api_rate_limit", Count: 3},
		},
	})
	soon := time.Now().UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	tlsUC := usecasetls.New(usecasetls.Config{CertDir: t.TempDir()})
	h := &SystemHandler{SystemDeps: &SystemDeps{
		loginLimiter:    lim,
		sessionRegistry: reg,
		logs:            logs,
		tlsUC:           tlsUC,
		users: postureUsers{items: []auth.UserPublic{
			{Username: "admin", Role: auth.RoleAdministrator},
			{Username: "op", Role: auth.RoleOperator, MustResetPassword: true},
		}},
		apiTokens: postureTokens{items: []auth.APITokenPublic{
			{ID: "t1", Name: "a", ExpiresAt: soon},
			{ID: "t2", Name: "b", ExpiresAt: ""},
			{ID: "t3", Name: "c", ExpiresAt: "not-a-date"},
		}},
	}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/system/security", nil)
	h.GetSecuritySummary(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out securitySummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.FailedLogins) == 0 {
		t.Fatal("expected failed logins")
	}
	if len(out.Sessions) != 1 {
		t.Fatalf("sessions=%d", len(out.Sessions))
	}
	if out.LoginPolicy.MaxFails != 3 {
		t.Fatalf("policy=%+v", out.LoginPolicy)
	}
	if out.Posture.Users.Total != 2 || out.Posture.Users.MustResetPassword != 1 {
		t.Fatalf("posture users=%+v", out.Posture.Users)
	}
	if out.Posture.Tokens.Total != 3 || out.Posture.Tokens.Expiring != 1 {
		t.Fatalf("posture tokens=%+v", out.Posture.Tokens)
	}
	if out.ThreatCounts.Last24h.CSRFReject != 2 || out.ThreatCounts.Last24h.APIRateLimit != 3 {
		t.Fatalf("threat=%+v", out.ThreatCounts.Last24h)
	}
}

func TestGetPutLoginPolicy(t *testing.T) {
	lim := loginthrottle.New(10, time.Minute, 5*time.Minute)
	dir := t.TempDir()
	path := dir + "/login_policy.json"
	h := &SystemHandler{SystemDeps: &SystemDeps{
		loginLimiter:    lim,
		loginPolicyFile: path,
		logs:            usecaseaudit.New(&fakeAuditStore{}),
	}}
	rec := httptest.NewRecorder()
	h.GetLoginPolicy(rec, httptest.NewRequest(http.MethodGet, "/api/system/login-policy", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d", rec.Code)
	}
	body := `{"max_fails":7,"window_sec":90,"lockout_sec":120}`
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/system/login-policy", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	h.PutLoginPolicy(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put %d %s", rec.Code, rec.Body.String())
	}
	p := lim.Policy()
	if p.MaxFails != 7 || p.WindowSec != 90 || p.LockoutSec != 120 {
		t.Fatalf("live policy=%+v", p)
	}
	loaded, err := LoadLoginPolicy(path)
	if err != nil || loaded.MaxFails != 7 {
		t.Fatalf("reload %+v err=%v", loaded, err)
	}
	def, err := LoadLoginPolicy("")
	if err != nil || def.MaxFails != defaultLoginMaxFails {
		t.Fatalf("defaults %+v err=%v", def, err)
	}
	missing, err := LoadLoginPolicy(dir + "/missing.json")
	if err != nil || missing.MaxFails != defaultLoginMaxFails {
		t.Fatalf("missing %+v err=%v", missing, err)
	}
	if err := validateLoginPolicy(LoginPolicyFile{MaxFails: 1, WindowSec: 90, LockoutSec: 120}); err == nil {
		t.Fatal("expected validate error")
	}
	bad := httptest.NewRecorder()
	badReq := httptest.NewRequest(http.MethodPut, "/api/system/login-policy", strings.NewReader(`{"max_fails":1,"window_sec":90,"lockout_sec":120}`))
	badReq.Header.Set("Content-Type", "application/json")
	h.PutLoginPolicy(bad, badReq)
	if bad.Code == http.StatusOK {
		t.Fatalf("expected rejection, got %d", bad.Code)
	}
}

func TestLoginPolicyHelpers(t *testing.T) {
	if got := normalizeLoginPolicy(LoginPolicyFile{}); got.MaxFails != defaultLoginMaxFails {
		t.Fatalf("normalize empty %+v", got)
	}
	if err := validateLoginPolicy(LoginPolicyFile{MaxFails: 5, WindowSec: 10, LockoutSec: 120}); err == nil {
		t.Fatal("window too small")
	}
	if err := validateLoginPolicy(LoginPolicyFile{MaxFails: 5, WindowSec: 60, LockoutSec: 10}); err == nil {
		t.Fatal("lockout too small")
	}
	if err := validateLoginPolicy(LoginPolicyFile{MaxFails: 5, WindowSec: 60, LockoutSec: 60}); err != nil {
		t.Fatal(err)
	}
	if err := saveLoginPolicy("", defaultLoginPolicy()); err == nil {
		t.Fatal("empty path")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "p.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLoginPolicy(path); err == nil {
		t.Fatal("corrupt json")
	}
}
