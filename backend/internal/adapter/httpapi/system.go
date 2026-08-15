package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"network_monitor/internal/adapter/httpapi/loginthrottle"
	usecasebackup "network_monitor/internal/usecase/backup"
	usecaseretention "network_monitor/internal/usecase/retention"
	"network_monitor/internal/usecase/system"
)

type systemStatsPayload struct {
	system.SystemStatsResponse
	FailedLogins []loginthrottle.FailedLoginEvent `json:"failed_logins"`
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
		FailedLogins:        h.failedLoginsSnapshot(),
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

func (h *SystemHandler) GetSystemVersion(w http.ResponseWriter, r *http.Request) {
	if h.systemUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "system service unavailable"})
		return
	}
	meta := h.systemUC.InstallMeta()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"version": meta.Version,
		"source":  meta.Source,
		"ref":     meta.Ref,
		"commit":  meta.Commit,
		"display": meta.Display,
	})
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

func (h *SystemHandler) GetBackups(w http.ResponseWriter, r *http.Request) {
	if h.backupUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "backup service unavailable"})
		return
	}
	cat, err := h.backupUC.Catalog()
	if err != nil {
		writeInternalError(w, "backup catalog failed", err)
		return
	}
	writeJSON(w, http.StatusOK, cat)
}

func (h *SystemHandler) PostBackup(w http.ResponseWriter, r *http.Request) {
	if h.backupUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "backup service unavailable"})
		return
	}
	err := h.backupUC.ScheduleCreate(r.Context(), usecasebackup.SourceManual)
	if err != nil {
		writeDomainError(w, "backup schedule failed", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok": true, "scheduled": true, "status": h.backupUC.Status(),
	})
}

func (h *SystemHandler) PostBackupAttach(w http.ResponseWriter, r *http.Request) {
	if h.backupUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "backup service unavailable"})
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	err := h.backupUC.ScheduleAttach(r.Context(), name)
	if err != nil {
		writeDomainError(w, "backup attach failed", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok": true, "scheduled": true, "action": "attach", "status": h.backupUC.Status(),
	})
}

func (h *SystemHandler) PostBackupDetach(w http.ResponseWriter, r *http.Request) {
	if h.backupUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "backup service unavailable"})
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	err := h.backupUC.ScheduleDetach(r.Context(), name)
	if err != nil {
		writeDomainError(w, "backup detach failed", err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok": true, "scheduled": true, "action": "detach", "status": h.backupUC.Status(),
	})
}

func (h *SystemHandler) DeleteBackup(w http.ResponseWriter, r *http.Request) {
	if h.backupUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "backup service unavailable"})
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	if err := h.backupUC.DeleteBackup(name); err != nil {
		writeDomainError(w, "backup delete failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": name})
}

func (h *SystemHandler) GetBackupSchedule(w http.ResponseWriter, r *http.Request) {
	if h.backupUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "backup service unavailable"})
		return
	}
	sch, err := h.backupUC.GetSchedule()
	if err != nil {
		writeInternalError(w, "backup schedule get failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "schedule": sch})
}

func (h *SystemHandler) PutBackupSchedule(w http.ResponseWriter, r *http.Request) {
	if h.backupUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "backup service unavailable"})
		return
	}
	var body struct {
		Enabled      *bool  `json:"enabled"`
		Hour         *int   `json:"hour"`
		Minute       *int   `json:"minute"`
		Timezone     string `json:"timezone"`
		Keep         *int   `json:"keep"`
		IncludeEdges *bool  `json:"include_edges"`
		IncludeAuth  *bool  `json:"include_auth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON"})
		return
	}
	cur, err := h.backupUC.GetSchedule()
	if err != nil {
		writeInternalError(w, "backup schedule get failed", err)
		return
	}
	if body.Enabled != nil {
		cur.Enabled = *body.Enabled
	}
	if body.Hour != nil {
		cur.Hour = *body.Hour
	}
	if body.Minute != nil {
		cur.Minute = *body.Minute
	}
	if strings.TrimSpace(body.Timezone) != "" {
		cur.Timezone = strings.TrimSpace(body.Timezone)
	}
	if body.Keep != nil {
		cur.Keep = *body.Keep
	}
	if body.IncludeEdges != nil {
		cur.IncludeEdges = *body.IncludeEdges
	}
	if body.IncludeAuth != nil {
		cur.IncludeAuth = *body.IncludeAuth
	}
	out, err := h.backupUC.UpdateSchedule(cur)
	if err != nil {
		writeDomainError(w, "backup schedule update failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "schedule": out})
}

func (h *SystemHandler) failedLoginsSnapshot() []loginthrottle.FailedLoginEvent {
	if h == nil || h.loginLimiter == nil {
		return []loginthrottle.FailedLoginEvent{}
	}
	out := h.loginLimiter.SnapshotFailures()
	if out == nil {
		return []loginthrottle.FailedLoginEvent{}
	}
	return out
}
