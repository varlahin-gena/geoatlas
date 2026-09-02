package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"geoatlas/internal/config"
)

func TestAPITreatMWRejectsPathTraversal(t *testing.T) {
	cfg := config.Config{APIRateLimitRPS: 0}
	h := apiThreatMW(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/../../etc/passwd", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAPITreatMWRateLimit(t *testing.T) {
	cfg := config.Config{APIRateLimitRPS: 1, APIRateLimitBurst: 1}
	h := apiThreatMW(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if i == 0 && rec.Code != http.StatusNoContent {
			t.Fatalf("first status = %d", rec.Code)
		}
		if i == 1 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("second status = %d, want 429", rec.Code)
		}
	}
}

func TestAPITreatMWSetsSecurityHeaders(t *testing.T) {
	cfg := config.Config{APIRateLimitRPS: 0}
	h := apiThreatMW(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff header")
	}
	if rec.Header().Get("Content-Security-Policy") != "default-src 'none'" {
		t.Fatal("missing API CSP header")
	}
}
