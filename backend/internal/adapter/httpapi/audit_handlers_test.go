package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domainauth "network_monitor/internal/auth"
	"network_monitor/internal/config"
	usecaseaudit "network_monitor/internal/usecase/auditlog"
	usecaseauth "network_monitor/internal/usecase/auth"
)

type fakeAuditStore struct {
	dr    []usecaseaudit.DREvent
	audit []usecaseaudit.AuditEvent
}

func (f *fakeAuditStore) WriteDR(_ context.Context, e usecaseaudit.DREvent) error {
	f.dr = append(f.dr, e)
	return nil
}

func (f *fakeAuditStore) WriteAudit(_ context.Context, e usecaseaudit.AuditEvent) error {
	f.audit = append(f.audit, e)
	return nil
}

func (f *fakeAuditStore) ListDR(_ context.Context, _ usecaseaudit.DRQuery) ([]usecaseaudit.DREvent, error) {
	return f.dr, nil
}

func (f *fakeAuditStore) ListAudit(_ context.Context, _ usecaseaudit.AuditQuery) ([]usecaseaudit.AuditEvent, error) {
	return f.audit, nil
}

func TestGetDRHistoryReturnsItems(t *testing.T) {
	logs := usecaseaudit.New(&fakeAuditStore{
		dr: []usecaseaudit.DREvent{{
			Timestamp: time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC),
			Actor:     "admin",
			Action:    "backup.create",
			Target:    "nm-1",
			Status:    "succeeded",
			Message:   "backup created",
		}},
	})
	h := &SystemHandler{SystemDeps: &SystemDeps{
		cfg:  config.Config{QueryTimeout: time.Second},
		logs: logs,
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/dr/history?limit=10", nil)
	rec := httptest.NewRecorder()

	h.GetDRHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []usecaseaudit.DREvent `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(body.Items) != 1 || body.Items[0].Action != "backup.create" {
		t.Fatalf("unexpected body: %+v", body.Items)
	}
}

func TestLoginFailedWritesAuditEvent(t *testing.T) {
	store, err := domainauth.NewUserStore()
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := domainauth.NewSessionManager("test-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeAuditStore{}
	h := &AuthHandler{AuthDeps: &AuthDeps{
		authUC: usecaseauth.New(store, sessions),
		logs:   usecaseaudit.New(fake),
	}}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"bad"}`))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if len(fake.audit) != 1 {
		t.Fatalf("audit events = %d, want 1", len(fake.audit))
	}
	if fake.audit[0].Action != "auth.login.failed" {
		t.Fatalf("action = %q", fake.audit[0].Action)
	}
}
