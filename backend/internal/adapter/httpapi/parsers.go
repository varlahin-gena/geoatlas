package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"geoatlas/internal/parser"
	usecaseaudit "geoatlas/internal/usecase/auditlog"
	"geoatlas/internal/usecase/parsers"
)

func (h *ParseHandler) ListParsers(w http.ResponseWriter, r *http.Request) {
	if h.parsersUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "parsers unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	items, err := h.parsersUC.List(ctx)
	if err != nil {
		writeDomainError(w, "list parsers failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "parsers": items})
}

func (h *ParseHandler) GetParser(w http.ResponseWriter, r *http.Request) {
	if h.parsersUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "parsers unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	item, err := h.parsersUC.Get(ctx, r.PathValue("id"))
	if err != nil {
		writeDomainError(w, "get parser failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "parser": item})
}

func (h *ParseHandler) CreateParser(w http.ResponseWriter, r *http.Request) {
	if h.parsersUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "parsers unavailable"})
		return
	}
	var in parsers.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	rec, err := h.parsersUC.Create(ctx, in)
	result := "succeeded"
	if err != nil {
		result = "failed"
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor: actorFromRequest(r), Action: "parsers.create",
		ResourceType: "parser", ResourceID: strings.TrimSpace(in.ID),
		Result: result, IP: clientIPFromRequest(r),
		Details: map[string]any{"error": errString(err)},
	})
	if err != nil {
		writeDomainError(w, "create parser failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "parser": rec})
}

func (h *ParseHandler) UpdateParser(w http.ResponseWriter, r *http.Request) {
	if h.parsersUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "parsers unavailable"})
		return
	}
	id := r.PathValue("id")
	var in parsers.UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	rec, err := h.parsersUC.Update(ctx, id, in)
	result := "succeeded"
	if err != nil {
		result = "failed"
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor: actorFromRequest(r), Action: "parsers.update",
		ResourceType: "parser", ResourceID: id,
		Result: result, IP: clientIPFromRequest(r),
		Details: map[string]any{"error": errString(err)},
	})
	if err != nil {
		writeDomainError(w, "update parser failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "parser": rec})
}

func (h *ParseHandler) DeleteParser(w http.ResponseWriter, r *http.Request) {
	if h.parsersUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "parsers unavailable"})
		return
	}
	id := r.PathValue("id")
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	err := h.parsersUC.Delete(ctx, id)
	result := "succeeded"
	if err != nil {
		result = "failed"
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor: actorFromRequest(r), Action: "parsers.delete",
		ResourceType: "parser", ResourceID: id,
		Result: result, IP: clientIPFromRequest(r),
		Details: map[string]any{"error": errString(err)},
	})
	if err != nil {
		writeDomainError(w, "delete parser failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *ParseHandler) ReorderParsers(w http.ResponseWriter, r *http.Request) {
	if h.parsersUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "parsers unavailable"})
		return
	}
	var body struct {
		Order []parsers.OrderEntry `json:"order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	if err := h.parsersUC.Reorder(ctx, body.Order); err != nil {
		writeDomainError(w, "reorder parsers failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *ParseHandler) DryRunParser(w http.ResponseWriter, r *http.Request) {
	if h.parsersUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "parsers unavailable"})
		return
	}
	var in parsers.DryRunInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()
	var live parser.ParseVerboseFunc
	if h.liveParse != nil {
		live = h.liveParse.ParseVerbose
	}
	result, err := h.parsersUC.DryRun(ctx, in, live)
	if err != nil {
		writeDomainError(w, "parser dry-run failed", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
