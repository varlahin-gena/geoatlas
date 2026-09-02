package threatprot

import "net/http"

// SetAPIResponseHeaders adds baseline security headers on JSON/API responses
// (Apigee AssignMessage security header policy).
func SetAPIResponseHeaders(w http.ResponseWriter) {
	if w == nil {
		return
	}
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Cache-Control", "no-store, no-cache, must-revalidate")
	h.Set("Content-Security-Policy", "default-src 'none'")
	h.Set("Referrer-Policy", "no-referrer")
}
