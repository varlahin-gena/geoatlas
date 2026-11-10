package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	usecaseaudit "geoatlas/internal/usecase/auditlog"
	"geoatlas/internal/usecase/connectors"
)

// ConnectorsHandler — CRUD + Apply for syslog connectors.
type ConnectorsHandler struct{ *ConnectorsDeps }

func (h *ConnectorsHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.connectorsUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "connectors not configured"})
		return
	}
	doc, err := h.connectorsUC.List()
	if err != nil {
		writeInternalError(w, "connectors list", err)
		return
	}
	pr := h.connectorsUC.PortRange()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"connectors":     doc.Connectors,
		"updated_at":     doc.UpdatedAt,
		"max_connectors": h.connectorsUC.MaxConnectors(),
		"extra_ports":    map[string]int{"min": pr.Min, "max": pr.Max},
		"default_port":   connectors.DefaultPort,
	})
}

func (h *ConnectorsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.connectorsUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "connectors not configured"})
		return
	}
	c, err := h.connectorsUC.Get(r.PathValue("id"))
	if err != nil {
		writeConnectorErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "connector": c})
}

func (h *ConnectorsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.connectorsUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "connectors not configured"})
		return
	}
	var in connectors.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	c, err := h.connectorsUC.Create(in)
	result := "succeeded"
	if err != nil {
		result = "failed"
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor: actorFromRequest(r), Action: "connectors.create",
		ResourceType: "connector", ResourceID: strings.TrimSpace(in.ID),
		Result: result, IP: clientIPFromRequest(r),
		Details: map[string]any{"port": in.Port, "protocols": in.Protocols, "error": errString(err)},
	})
	if err != nil {
		writeConnectorErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "connector": c})
}

func (h *ConnectorsHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.connectorsUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "connectors not configured"})
		return
	}
	id := r.PathValue("id")
	var in connectors.UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	c, err := h.connectorsUC.Update(id, in)
	result := "succeeded"
	if err != nil {
		result = "failed"
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor: actorFromRequest(r), Action: "connectors.update",
		ResourceType: "connector", ResourceID: id,
		Result: result, IP: clientIPFromRequest(r),
		Details: map[string]any{"error": errString(err)},
	})
	if err != nil {
		writeConnectorErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "connector": c})
}

func (h *ConnectorsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.connectorsUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "connectors not configured"})
		return
	}
	id := r.PathValue("id")
	err := h.connectorsUC.Delete(id)
	result := "succeeded"
	if err != nil {
		result = "failed"
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor: actorFromRequest(r), Action: "connectors.delete",
		ResourceType: "connector", ResourceID: id,
		Result: result, IP: clientIPFromRequest(r),
		Details: map[string]any{"error": errString(err)},
	})
	if err != nil {
		writeConnectorErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": id})
}

func (h *ConnectorsHandler) Apply(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.connectorsUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "connectors not configured"})
		return
	}
	res, err := h.connectorsUC.Apply(r.Context())
	result := "succeeded"
	if err != nil || !res.OK {
		result = "failed"
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor: actorFromRequest(r), Action: "connectors.apply",
		ResourceType: "connector", ResourceID: "all",
		Result: result, IP: clientIPFromRequest(r),
		Details: map[string]any{
			"reloaded": res.Reloaded, "reload_error": res.ReloadError,
			"enabled": res.EnabledCount, "error": errString(err),
		},
	})
	if err != nil {
		// Surface write/path errors to admin UI (permission denied on syslog-ng.d is common).
		slog.Error("connectors apply", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "connectors apply failed: " + err.Error(),
		})
		return
	}
	status := http.StatusOK
	if !res.OK {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, map[string]any{"ok": res.OK, "apply": res})
}

func writeConnectorErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, connectors.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
	case errors.Is(err, connectors.ErrBuiltinDelete),
		errors.Is(err, connectors.ErrIDTaken),
		errors.Is(err, connectors.ErrPortTaken),
		errors.Is(err, connectors.ErrPortNotAllowed),
		errors.Is(err, connectors.ErrInvalid),
		errors.Is(err, connectors.ErrLimitReached):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	default:
		writeInternalError(w, "connectors", err)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
