package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	usecaseanomaly "network_monitor/internal/usecase/anomaly"
)

type AnomalyHandler struct{ *AnomalyDeps }

func (h *AnomalyHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.anomalyUC == nil || !h.anomalyUC.Enabled() {
		writeJSON(w, http.StatusOK, usecaseanomaly.ListResult{
			Items:   []usecaseanomaly.Event{},
			Summary: usecaseanomaly.Summary{Enabled: false},
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
	if h == nil || h.anomalyUC == nil || !h.anomalyUC.Enabled() {
		writeJSON(w, http.StatusOK, usecaseanomaly.Summary{Enabled: false, UpdatedAt: time.Now().UTC()})
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
	if h == nil || h.anomalyUC == nil || !h.anomalyUC.Enabled() {
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
	if h == nil || h.anomalyUC == nil || !h.anomalyUC.Enabled() {
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

func (h *AnomalyHandler) actorName(r *http.Request) string {
	if h != nil && h.cfg.AuthDisabled {
		return "anonymous"
	}
	if sess, ok := SessionFromContext(r.Context()); ok && strings.TrimSpace(sess.Username) != "" {
		return sess.Username
	}
	return "bearer"
}
