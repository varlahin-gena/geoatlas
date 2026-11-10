package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"geoatlas/internal/adapter/httpapi/authmw"
	"geoatlas/internal/auth"
)

func TestMapQueryGateSecondBeginCancelsFirst(t *testing.T) {
	g := NewMapQueryGate()
	parent := context.Background()

	ctx1, done1 := g.Begin("user:a", parent)
	ctx2, done2 := g.Begin("user:a", parent)
	defer done2()

	select {
	case <-ctx1.Done():
		// expected: superseded
	case <-time.After(time.Second):
		t.Fatal("first ctx was not cancelled")
	}
	if ctx2.Err() != nil {
		t.Fatalf("second ctx already done: %v", ctx2.Err())
	}

	done1() // superseded done must be safe
	if ctx2.Err() != nil {
		t.Fatalf("done1 must not cancel second ctx: %v", ctx2.Err())
	}

	done2()
	select {
	case <-ctx2.Done():
	case <-time.After(time.Second):
		t.Fatal("second ctx not cancelled by done2")
	}
}

func TestMapQueryGateIsolatesKeys(t *testing.T) {
	g := NewMapQueryGate()
	ctxA, doneA := g.Begin("user:a", context.Background())
	ctxB, doneB := g.Begin("user:b", context.Background())
	defer doneA()
	defer doneB()

	if ctxA.Err() != nil || ctxB.Err() != nil {
		t.Fatal("independent keys must not cancel each other")
	}
}

func TestMapQueryGateKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.RemoteAddr = "1.2.3.4:9999"
	if got := mapQueryGateKey(req); got != "anon:1.2.3.4:9999" {
		t.Fatalf("anon key=%q", got)
	}

	req2 := req.WithContext(authmw.ContextWithSession(req.Context(), auth.Session{Username: "bob", SID: "s1"}))
	if got := mapQueryGateKey(req2); got != "sid:s1" {
		t.Fatalf("sid key=%q", got)
	}

	req3 := req.WithContext(authmw.ContextWithSession(req.Context(), auth.Session{Username: "bob"}))
	if got := mapQueryGateKey(req3); got != "user:bob" {
		t.Fatalf("user key=%q", got)
	}
}
