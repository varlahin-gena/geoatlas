package httpapi

import (
	"testing"
	"time"
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
