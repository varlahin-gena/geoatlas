package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	usecaseanomaly "geoatlas/internal/usecase/anomaly"
	usecaseaudit "geoatlas/internal/usecase/auditlog"
)

type AnomalyHandler struct{ *AnomalyDeps }

func (h *AnomalyHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.anomalyUC == nil || !h.anomalyUC.Available() {
		writeJSON(w, http.StatusOK, usecaseanomaly.ListResult{
			Items:   []usecaseanomaly.Event{},
			Summary: usecaseanomaly.Summary{Enabled: false, ModuleLoaded: false},
		})
		return
	}
	q := usecaseanomaly.ListQuery{
		IncludeAcked: r.URL.Query().Get("include_acked") == "1" || r.URL.Query().Get("include_acked") == "true",
		Severity:     strings.TrimSpace(r.URL.Query().Get("severity")),
		Code:         strings.TrimSpace(r.URL.Query().Get("code")),
		Limit:        50,
	}
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			q.Limit = n
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("since")); v != "" {
		t, err := parseTimeParam(v)
		if err != nil {
			writeBadRequest(w, "invalid since")
			return
		}
		q.Since = t
	}
	out, err := h.anomalyUC.List(r.Context(), q)
	if err != nil {
		writeInternalError(w, "anomaly list failed", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *AnomalyHandler) Summary(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.anomalyUC == nil || !h.anomalyUC.Available() {
		writeJSON(w, http.StatusOK, usecaseanomaly.Summary{Enabled: false, ModuleLoaded: false, UpdatedAt: time.Now().UTC()})
		return
	}
	out, err := h.anomalyUC.Summary(r.Context())
	if err != nil {
		writeInternalError(w, "anomaly summary failed", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *AnomalyHandler) Status(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.anomalyUC == nil {
		writeJSON(w, http.StatusOK, usecaseanomaly.ScanStatus{Enabled: false})
		return
	}
	writeJSON(w, http.StatusOK, h.anomalyUC.Status())
}

func (h *AnomalyHandler) Ack(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.anomalyUC == nil || !h.anomalyUC.Available() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "anomaly module disabled"})
		return
	}
	fp := strings.TrimSpace(r.PathValue("fingerprint"))
	if fp == "" {
		writeBadRequest(w, "missing fingerprint")
		return
	}
	by := h.actorName(r)
	if err := h.anomalyUC.Ack(r.Context(), fp, by); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "fingerprint": fp, "ack_by": by})
}

type assignAnomalyRequest struct {
	AssignedTo string `json:"assigned_to"`
}

func (h *AnomalyHandler) Assign(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.anomalyUC == nil || !h.anomalyUC.Available() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "anomaly module disabled"})
		return
	}
	fp := strings.TrimSpace(r.PathValue("fingerprint"))
	if fp == "" {
		writeBadRequest(w, "missing fingerprint")
		return
	}
	var req assignAnomalyRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeBadRequest(w, "invalid json")
		return
	}
	by := h.actorName(r)
	if err := h.anomalyUC.Assign(r.Context(), fp, req.AssignedTo, by); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "fingerprint": fp, "assigned_to": strings.TrimSpace(req.AssignedTo),
	})
}

func (h *AnomalyHandler) Episodes(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.anomalyUC == nil || !h.anomalyUC.Available() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "episodes": []usecaseanomaly.EpisodeSummary{}})
		return
	}
	q := usecaseanomaly.ListQuery{IncludeAcked: r.URL.Query().Get("include_acked") == "1", Limit: 200}
	if v := strings.TrimSpace(r.URL.Query().Get("since")); v != "" {
		t, err := parseTimeParam(v)
		if err != nil {
			writeBadRequest(w, "invalid since")
			return
		}
		q.Since = t
	}
	out, err := h.anomalyUC.Episodes(r.Context(), q)
	if err != nil {
		writeInternalError(w, "anomaly episodes failed", err)
		return
	}
	if out == nil {
		out = []usecaseanomaly.EpisodeSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "episodes": out})
}

func (h *AnomalyHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.anomalySettings == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "anomaly module unavailable"})
		return
	}
	view, err := h.anomalySettings.GetView()
	if err != nil {
		writeInternalError(w, "anomaly settings get failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"settings":        view.Settings,
		"install_profile": view.InstallProfile,
		"thresholds":      view.Thresholds,
		"status":          view.Status,
	})
}

func (h *AnomalyHandler) PutSettings(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.anomalySettings == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "anomaly module unavailable"})
		return
	}
	var req usecaseanomaly.Settings
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeBadRequest(w, "invalid json")
		return
	}
	timeout := h.cfg.QueryTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	view, err := h.anomalySettings.Update(ctx, req)
	if err != nil {
		writeAuditEvent(ctx, h.logs, usecaseaudit.AuditEvent{
			Actor:        actorFromRequest(r),
			Action:       "anomalies.settings.update",
			ResourceType: "anomaly_settings",
			ResourceID:   "engine",
			Result:       "failed",
			IP:           clientIPFromRequest(r),
			Details:      map[string]any{"error": err.Error()},
		})
		writeDomainError(w, "anomaly settings update failed", err)
		return
	}
	writeAuditEvent(ctx, h.logs, usecaseaudit.AuditEvent{
		Actor:        actorFromRequest(r),
		Action:       "anomalies.settings.update",
		ResourceType: "anomaly_settings",
		ResourceID:   "engine",
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
		Details: map[string]any{
			"enabled":              view.Settings.Enabled,
			"scan_interval_min":    view.Settings.ScanIntervalMin,
			"learning_days":        view.Settings.LearningDays,
			"suppress_hours":       view.Settings.SuppressHours,
			"include_private":      view.Settings.IncludePrivate,
			"new_country_min_share": view.Settings.NewCountryMinShare,
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"settings":        view.Settings,
		"install_profile": view.InstallProfile,
		"thresholds":      view.Thresholds,
		"status":          view.Status,
	})
}

func (h *AnomalyHandler) actorName(r *http.Request) string {
	if h != nil && h.cfg.AuthDisabled {
		return "anonymous"
	}
	if sess, ok := SessionFromContext(r.Context()); ok && strings.TrimSpace(sess.Username) != "" {
		return sess.Username
	}
	return "bearer"
}
