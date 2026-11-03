package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"geoatlas/internal/adapter/httpapi/loginthrottle"
	"geoatlas/internal/auth"
	usecaseaudit "geoatlas/internal/usecase/auditlog"
)

func TestGetSecuritySummary(t *testing.T) {
	reg, err := auth.NewSessionRegistry("")
	if err != nil {
		t.Fatal(err)
	}
	_ = reg.Register(auth.SessionRecord{ID: "s1", Username: "admin", Role: auth.RoleAdministrator, IP: "127.0.0.1"})
	lim := loginthrottle.New(3, time.Minute, time.Minute)
	lim.RecordFailure("9.9.9.9", "eve")
	logs := usecaseaudit.New(&fakeAuditStore{
		audit: []usecaseaudit.AuditEvent{{
			Timestamp: time.Now().UTC(),
			Actor:     "admin",
			Action:    "auth.login.success",
			Result:    "succeeded",
		}},
	})
	h := &SystemHandler{SystemDeps: &SystemDeps{
		loginLimiter:    lim,
		sessionRegistry: reg,
		logs:            logs,
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
}

func TestGetPutLoginPolicy(t *testing.T) {
	lim := loginthrottle.New(10, time.Minute, 5*time.Minute)
	dir := t.TempDir()
	h := &SystemHandler{SystemDeps: &SystemDeps{
		loginLimiter:    lim,
		loginPolicyFile: dir + "/login_policy.json",
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
}
