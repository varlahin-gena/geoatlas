package httpapi

import (
	"context"
	"testing"
	"time"

	usecaseaudit "geoatlas/internal/usecase/auditlog"
)

func TestRateLimitCoalesce(t *testing.T) {
	c := newRateLimitCoalesce()
	now := time.Unix(1_700_000_000, 0)
	emit, n := c.shouldEmit("1.1.1.1", now)
	if !emit || n != 1 {
		t.Fatalf("first emit=%v n=%d", emit, n)
	}
	emit, n = c.shouldEmit("1.1.1.1", now.Add(10*time.Second))
	if emit {
		t.Fatalf("should coalesce, n=%d", n)
	}
	emit, n = c.shouldEmit("1.1.1.1", now.Add(time.Minute))
	if !emit || n < 1 {
		t.Fatalf("after window emit=%v n=%d", emit, n)
	}
}

func TestAuditSecurityRejectAndRateLimit(t *testing.T) {
	store := &fakeAuditStore{}
	logs := usecaseaudit.New(store)
	auditSecurityReject(context.Background(), logs, "security.csrf.reject", "1.2.3.4", "/api/x", "bad", nil)
	auditAPIRateLimit(context.Background(), logs, "", "/api/y")
	auditAPIRateLimit(context.Background(), logs, "", "/api/y") // coalesced
	if len(store.audit) < 1 {
		t.Fatalf("expected audit rows, got %d", len(store.audit))
	}
}
