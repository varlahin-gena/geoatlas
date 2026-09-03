package httpapi

import (
	"net/http"

	"geoatlas/internal/adapter/httpapi/loginthrottle"
	"geoatlas/internal/adapter/httpapi/threatprot"
	"geoatlas/internal/config"
)

// apiThreatMW applies Apigee-equivalent edge controls: injection guard, SpikeArrest,
// and API security response headers.
func apiThreatMW(cfg config.Config) middleware {
	limiter := threatprot.NewRateLimiter(cfg.APIRateLimitRPS, cfg.APIRateLimitBurst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			threatprot.SetAPIResponseHeaders(w)

			if threatprot.SuspiciousRequest(r) {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "malformed request"})
				return
			}

			if !rateLimitExempt(r.URL.Path) {
				key := threatprot.RateLimitKey(r, loginthrottle.ClientIP)
				if !limiter.Allow(key) {
					w.Header().Set("Retry-After", "1")
					writeJSON(w, http.StatusTooManyRequests, map[string]any{
						"error":   "rate limit exceeded",
						"details": map[string]any{"reason": "rate_limited"},
					})
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitExempt(path string) bool {
	switch path {
	case "/live", "/ready", "/metrics",
		"/api/live", "/api/ready", "/api/health",
		"/api/auth/check", "/api/auth/check-admin", "/api/auth/check-ops":
		return true
	default:
		return false
	}
}
