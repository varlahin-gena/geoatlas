package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"network_monitor/internal/usecase/system"
	usecaseretention "network_monitor/internal/usecase/retention"
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

func (h *SystemHandler) GetRetention(w http.ResponseWriter, r *http.Request) {
	if h.retentionUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "retention service unavailable"})
		return
	}
	settings, err := h.retentionUC.Get()
	if err != nil {
		writeInternalError(w, "retention get failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "retention": settings})
}

func (h *SystemHandler) PutRetention(w http.ResponseWriter, r *http.Request) {
	if h.retentionUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "retention service unavailable"})
		return
	}
	var req usecaseretention.Settings
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	timeout := h.cfg.QueryTimeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	out, err := h.retentionUC.Update(ctx, req)
	if err != nil {
		writeDomainError(w, "retention update failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "retention": out})
}

func failedLoginsSnapshot() []FailedLoginEvent {
	out := defaultLoginLimiter.snapshotFailures()
	if out == nil {
		return []FailedLoginEvent{}
	}
	return out
}
