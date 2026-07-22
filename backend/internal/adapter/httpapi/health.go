package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Health — публичный liveness для docker/k8s.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	if h.systemUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok": false, "status": "unavailable", "error": "system service unavailable",
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	result, err := h.systemUC.Health(ctx, h.systemPinger)
	if err != nil {
		slog.Error("health failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok": false, "status": "unavailable", "error": "health check failed",
		})
		return
	}
	if !result.OK {
		slog.Error("health: clickhouse unavailable")
	}
	writeJSON(w, result.HTTPStatus, result.Body)
}
