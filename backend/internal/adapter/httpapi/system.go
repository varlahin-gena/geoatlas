package httpapi

import (
	"context"
	"net/http"

	"network_monitor/internal/usecase/system"
)

type systemStatsPayload struct {
	system.SystemStatsResponse
	FailedLogins []FailedLoginEvent `json:"failed_logins"`
}

func (h *SystemHandler) GetSystemStats(w http.ResponseWriter, r *http.Request) {
	if h.systemUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "system service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.QueryTimeout)
	defer cancel()

	resp, err := h.systemUC.CollectStats(ctx)
	if err != nil {
		writeInternalError(w, "system stats: fetch metrics failed", err)
		return
	}
	writeJSON(w, http.StatusOK, systemStatsPayload{
		SystemStatsResponse: resp,
		FailedLogins:        failedLoginsSnapshot(),
	})
}

func (h *SystemHandler) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	if h.systemUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "system service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.QueryTimeout)
	defer cancel()

	resp, err := h.systemUC.Status(ctx)
	if err != nil {
		writeInternalError(w, "system status: fetch metrics failed", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *SystemHandler) GetSystemHistory(w http.ResponseWriter, r *http.Request) {
	if h.systemUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "system service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.QueryTimeout)
	defer cancel()

	q := r.URL.Query()
	resp, err := h.systemUC.History(ctx, q.Get("period"), q.Get("metrics"))
	if err != nil {
		writeInternalError(w, "system history: fetch failed", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *SystemHandler) GetEdgesAggStatus(w http.ResponseWriter, r *http.Request) {
	if h.systemUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "system service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), healthTimeout)
	defer cancel()
	writeJSON(w, http.StatusOK, h.systemUC.EdgesAgg(ctx))
}

func (h *SystemHandler) PostMaintenanceBackfill(w http.ResponseWriter, r *http.Request) {
	if h.systemUC == nil || !h.systemUC.ScheduleMaintenanceBackfill(r.Context()) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "maintenance scheduler unavailable",
		})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok": true, "scheduled": true, "job": "edges_geo_backfill", "status": "/api/system/edges-agg",
	})
}

func (h *SystemHandler) GetInstallProfile(w http.ResponseWriter, r *http.Request) {
	if h.systemUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "system service unavailable"})
		return
	}
	profile, err := h.systemUC.InstallProfile()
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "install profile not found"})
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func failedLoginsSnapshot() []FailedLoginEvent {
	out := defaultLoginLimiter.snapshotFailures()
	if out == nil {
		return []FailedLoginEvent{}
	}
	return out
}
