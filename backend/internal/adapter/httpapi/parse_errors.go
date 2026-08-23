package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"network_monitor/internal/usecase/parseerrors"
)

func (h *ParseHandler) ListParseErrors(w http.ResponseWriter, r *http.Request) {
	if h.parseErrorsUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "parse errors service unavailable"})
		return
	}
	q := r.URL.Query()
	limit := 500
	if n, err := strconv.Atoi(q.Get("limit")); err == nil {
		limit = n
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.QueryTimeout)
	defer cancel()

	result, err := h.parseErrorsUC.List(ctx, parseerrors.ListInput{
		Limit:  limit,
		Search: strings.TrimSpace(q.Get("search")),
	})
	if err != nil {
		writeInternalError(w, "list parse errors failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "count": len(result.Items), "errors": result.Items,
	})
}

func (h *ParseHandler) DeleteParseErrors(w http.ResponseWriter, r *http.Request) {
	if h.parseErrorsUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "parse errors service unavailable"})
		return
	}
	defer r.Body.Close()

	var req struct {
		IDs []string `json:"ids"`
		All bool     `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.QueryTimeout)
	defer cancel()

	err := h.parseErrorsUC.Delete(ctx, parseerrors.DeleteInput{IDs: req.IDs, All: req.All})
	if err != nil {
		writeDomainError(w, "delete parse errors failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
