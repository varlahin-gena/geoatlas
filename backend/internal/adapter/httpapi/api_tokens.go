package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"network_monitor/internal/auth"
	usecaseaudit "network_monitor/internal/usecase/auditlog"
)

// APITokensHandler — CRUD именованных Bearer-токенов со scope.
type APITokensHandler struct{ *AuthDeps }

func (h *APITokensHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.apiTokens == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "api tokens not configured"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": h.apiTokens.List()})
}

func (h *APITokensHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.apiTokens == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "api tokens not configured"})
		return
	}
	var body struct {
		Name  string `json:"name"`
		Scope string `json:"scope"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	pub, plain, err := h.apiTokens.Create(body.Name, body.Scope)
	if err != nil {
		writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
			Actor:        actorFromRequest(r),
			Action:       "auth.token.create",
			ResourceType: "api_token",
			ResourceID:   body.Name,
			Result:       "failed",
			IP:           clientIPFromRequest(r),
			Details:      map[string]any{"error": err.Error(), "scope": body.Scope},
		})
		writeTokenStoreError(w, err)
		return
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor:        actorFromRequest(r),
		Action:       "auth.token.create",
		ResourceType: "api_token",
		ResourceID:   pub.ID,
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
		Details:      map[string]any{"name": pub.Name, "scope": pub.Scope},
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":  pub,
		"secret": plain, // plaintext только при создании
	})
}

func (h *APITokensHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.apiTokens == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "api tokens not configured"})
		return
	}
	id := r.PathValue("id")
	if err := h.apiTokens.Revoke(id); err != nil {
		writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
			Actor:        actorFromRequest(r),
			Action:       "auth.token.revoke",
			ResourceType: "api_token",
			ResourceID:   id,
			Result:       "failed",
			IP:           clientIPFromRequest(r),
			Details:      map[string]any{"error": err.Error()},
		})
		writeTokenStoreError(w, err)
		return
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor:        actorFromRequest(r),
		Action:       "auth.token.revoke",
		ResourceType: "api_token",
		ResourceID:   id,
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeTokenStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidTokenName), errors.Is(err, auth.ErrInvalidScope):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	case errors.Is(err, auth.ErrTokenNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
	case errors.Is(err, auth.ErrTokenLimit):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
	default:
		writeInternalError(w, "api tokens", err)
	}
}
