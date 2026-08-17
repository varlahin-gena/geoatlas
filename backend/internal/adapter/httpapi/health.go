package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Live — процесс жив. Без ClickHouse и ingest (docker/k8s liveness).
// /health и /api/health тоже зовут Live.
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": "live"})
}

// Ready — ClickHouse ping + снимок ingest. HTTP 503 только если CH недоступен.
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
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
		slog.Error("ready failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok": false, "status": "unavailable", "error": "ready check failed",
		})
		return
	}
	if !result.OK {
		slog.Error("ready: clickhouse unavailable")
		writeJSON(w, http.StatusServiceUnavailable, result.Body)
		return
	}
	writeJSON(w, http.StatusOK, result.Body)
}
