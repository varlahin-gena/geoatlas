package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"geoatlas/internal/adapter/httpapi/loginthrottle"
)

func TestProxyGateMW(t *testing.T) {
	loginthrottle.ConfigureTrustedProxies([]string{"10.0.0.1"})
	t.Cleanup(func() { loginthrottle.ConfigureTrustedProxies([]string{"frontend"}) })

	ba := newBearerAuth([]string{"env-admin-secret"}, nil, nil)
	okH := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	gated := proxyGateMW(ba, true)(okH)
	open := proxyGateMW(ba, false)(okH)

	t.Run("disabled allows external", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/live", nil)
		req.RemoteAddr = "203.0.113.9:443"
		rec := httptest.NewRecorder()
		open.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("rejects external without bearer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/live", nil)
		req.RemoteAddr = "203.0.113.9:443"
		rec := httptest.NewRecorder()
		gated.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status=%d want 403", rec.Code)
		}
	})

	t.Run("allows trusted hop", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/live", nil)
		req.RemoteAddr = "10.0.0.1:80"
		rec := httptest.NewRecorder()
		gated.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("allows bearer from external", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ingest/stats", nil)
		req.RemoteAddr = "203.0.113.9:443"
		req.Header.Set("Authorization", "Bearer env-admin-secret")
		rec := httptest.NewRecorder()
		gated.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}
