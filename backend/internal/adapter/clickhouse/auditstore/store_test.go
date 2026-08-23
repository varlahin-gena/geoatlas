package auditstore

import (
	"context"
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
}

func TestListDRNilCH(t *testing.T) {
	r := &Repository{}
	_, err := r.ListDR(context.Background(), usecaseaudit.DRQuery{Since: time.Now()})
	if err == nil {
		t.Fatal("want error")
	}
}

func TestListAuditNilCH(t *testing.T) {
	r := &Repository{}
	_, err := r.ListAudit(context.Background(), usecaseaudit.AuditQuery{Since: time.Now()})
	if err == nil {
		t.Fatal("want error")
	}
}
