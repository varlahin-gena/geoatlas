package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddlewareGeneratesAndEchoes(t *testing.T) {
	var got string
	h := requestIDMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(rr, req)

	if got == "" || len(got) < 8 {
		t.Fatalf("request id in context: %q", got)
	}
	if hdr := rr.Header().Get("X-Request-ID"); hdr != got {
		t.Fatalf("response header %q != context %q", hdr, got)
	}
}

func TestRequestIDMiddlewareAcceptsIncoming(t *testing.T) {
	const want = "client-trace-abc123"
	var got string
	h := requestIDMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-ID", want)
	h.ServeHTTP(rr, req)

	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if rr.Header().Get("X-Request-ID") != want {
		t.Fatalf("echo header %q", rr.Header().Get("X-Request-ID"))
	}
}

func TestIsDryRun(t *testing.T) {
	cases := []struct {
		q    string
		want bool
	}{
		{"", false},
		{"dry_run=1", true},
		{"dry_run=true", true},
		{"dry_run=0", false},
		{"dry_run=no", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/upload-geo?"+tc.q, nil)
		if got := isDryRun(req); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.q, got, tc.want)
		}
	}
}
