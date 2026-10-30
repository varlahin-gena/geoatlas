package auditstore

import (
	"context"
	"errors"
	"testing"
	"time"

	usecaseaudit "geoatlas/internal/usecase/auditlog"
)

func TestNormalizeListLimit(t *testing.T) {
	tests := []struct {
		in, want int
	}{
		{0, 100},
		{-1, 100},
		{201, 100},
		{50, 50},
	}
	for _, tc := range tests {
		if got := normalizeListLimit(tc.in); got != tc.want {
			t.Fatalf("normalizeListLimit(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestWriteDRNilGuard(t *testing.T) {
	var r *Repository
	if err := r.WriteDR(context.Background(), usecaseaudit.DREvent{
		Timestamp: time.Now(),
		Actor:     "admin",
		Action:    "test",
	}); err != nil {
		t.Fatalf("nil repo: %v", err)
	}
	r = &Repository{}
	if err := r.WriteDR(context.Background(), usecaseaudit.DREvent{}); err != nil {
		t.Fatalf("nil ch: %v", err)
	}
}

func TestWriteAuditNilGuard(t *testing.T) {
	var r *Repository
	if err := r.WriteAudit(context.Background(), usecaseaudit.AuditEvent{}); err != nil {
		t.Fatalf("nil repo: %v", err)
	}
	r = &Repository{}
	if err := r.WriteAudit(context.Background(), usecaseaudit.AuditEvent{}); err != nil {
		t.Fatalf("nil ch: %v", err)
	}
}

func TestListDRNilCH(t *testing.T) {
	r := &Repository{}
	_, err := r.ListDR(context.Background(), usecaseaudit.DRQuery{Since: time.Now()})
	if err == nil {
		t.Fatal("want error")
	}
	var nilR *Repository
	_, err = nilR.ListDR(context.Background(), usecaseaudit.DRQuery{})
	if err == nil {
		t.Fatal("want nil repo error")
	}
}

func TestListAuditNilCH(t *testing.T) {
	r := &Repository{}
	_, err := r.ListAudit(context.Background(), usecaseaudit.AuditQuery{Since: time.Now()})
	if err == nil {
		t.Fatal("want error")
	}
	var nilR *Repository
	_, err = nilR.ListAudit(context.Background(), usecaseaudit.AuditQuery{})
	if err == nil {
		t.Fatal("want nil repo error")
	}
}

func TestNewAndWriteWithMeta(t *testing.T) {
	ch := &stubConn{}
	r := New(ch)
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := r.WriteDR(context.Background(), usecaseaudit.DREvent{
		Timestamp: now, Actor: "a", Action: "x", Target: "t", Status: "ok", Message: "m",
		Meta: map[string]any{"k": "v"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteDR(context.Background(), usecaseaudit.DREvent{Timestamp: now}); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteAudit(context.Background(), usecaseaudit.AuditEvent{
		Timestamp: now, Actor: "a", Action: "login", ResourceType: "user", ResourceID: "1",
		Result: "ok", IP: "1.1.1.1", Details: map[string]any{"n": 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteAudit(context.Background(), usecaseaudit.AuditEvent{Timestamp: now}); err != nil {
		t.Fatal(err)
	}
	if len(ch.execCalls) != 4 {
		t.Fatalf("execCalls=%d", len(ch.execCalls))
	}
}

func TestWriteExecError(t *testing.T) {
	ch := &stubConn{execErr: errors.New("boom")}
	r := New(ch)
	if err := r.WriteDR(context.Background(), usecaseaudit.DREvent{}); err == nil {
		t.Fatal("want exec err")
	}
	if err := r.WriteAudit(context.Background(), usecaseaudit.AuditEvent{}); err == nil {
		t.Fatal("want exec err")
	}
}

func TestListDRFiltersAndMeta(t *testing.T) {
	ch := &stubConn{}
	now := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	ch.push([]any{now, "admin", "backup", "db", "ok", "done", `{"x":1}`})
	ch.push([]any{now, "u", "a", "t", "fail", "m", `{}`})
	r := New(ch)
	out, err := r.ListDR(context.Background(), usecaseaudit.DRQuery{
		Since: now.Add(-time.Hour), Limit: 10, Action: "backup", Status: "ok", Actor: "admin",
	})
	if err != nil || len(out) != 1 || out[0].Meta["x"].(float64) != 1 {
		t.Fatalf("%+v %v", out, err)
	}
	out, err = r.ListDR(context.Background(), usecaseaudit.DRQuery{Since: now, Limit: 0})
	if err != nil || len(out) != 1 || out[0].Meta != nil {
		t.Fatalf("%+v %v", out, err)
	}
}

func TestListDRQueryErrorAndScanError(t *testing.T) {
	ch := &stubConn{}
	ch.pushErr(errors.New("q fail"))
	r := New(ch)
	if _, err := r.ListDR(context.Background(), usecaseaudit.DRQuery{Since: time.Now()}); err == nil {
		t.Fatal("want query err")
	}
	ch.push([]any{"bad"}) // wrong arity → scan err
	if _, err := r.ListDR(context.Background(), usecaseaudit.DRQuery{Since: time.Now()}); err == nil {
		t.Fatal("want scan err")
	}
}

func TestListAuditFiltersAndDetails(t *testing.T) {
	ch := &stubConn{}
	now := time.Date(2024, 5, 2, 0, 0, 0, 0, time.UTC)
	ch.push([]any{now, "admin", "update", "hunt", "h1", "ok", "9.9.9.9", `{"z":true}`})
	ch.push([]any{now, "u", "a", "r", "id", "fail", "", ""})
	r := New(ch)
	out, err := r.ListAudit(context.Background(), usecaseaudit.AuditQuery{
		Since: now.Add(-time.Hour), Limit: 20, Action: "update", Result: "ok", Actor: "admin",
	})
	if err != nil || len(out) != 1 || out[0].Details["z"] != true {
		t.Fatalf("%+v %v", out, err)
	}
	out, err = r.ListAudit(context.Background(), usecaseaudit.AuditQuery{Since: now, Limit: 999})
	if err != nil || len(out) != 1 {
		t.Fatalf("%+v %v", out, err)
	}
}

func TestListAuditQueryErrorAndScanError(t *testing.T) {
	ch := &stubConn{}
	ch.pushErr(errors.New("q fail"))
	r := New(ch)
	if _, err := r.ListAudit(context.Background(), usecaseaudit.AuditQuery{Since: time.Now()}); err == nil {
		t.Fatal("want query err")
	}
	ch.push([]any{"bad"})
	if _, err := r.ListAudit(context.Background(), usecaseaudit.AuditQuery{Since: time.Now()}); err == nil {
		t.Fatal("want scan err")
	}
}
