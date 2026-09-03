package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"geoatlas/internal/auth"
	usecaseaudit "geoatlas/internal/usecase/auditlog"
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
		Name            string `json:"name"`
		Scope           string `json:"scope"`
		ExpiresAt       string `json:"expires_at"`
		CurrentPassword string `json:"current_password"`
	}
	if !decodeJSONBody(w, r, &body, defaultJSONBodyLimit) {
		return
	}
	if _, ok := h.reauth.Require(w, r, body.CurrentPassword); !ok {
		return
	}
	expiresAt, err := parseOptionalExpiry(body.ExpiresAt)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	pub, plain, err := h.apiTokens.Create(body.Name, body.Scope, expiresAt)
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
	details := map[string]any{"name": pub.Name, "scope": pub.Scope}
	if pub.ExpiresAt != "" {
		details["expires_at"] = pub.ExpiresAt
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor:        actorFromRequest(r),
		Action:       "auth.token.create",
		ResourceType: "api_token",
		ResourceID:   pub.ID,
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
		Details:      details,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":  pub,
		"secret": plain, // plaintext только при создании
	})
}

func (h *APITokensHandler) Rotate(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.apiTokens == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "api tokens not configured"})
		return
	}
	id := r.PathValue("id")
	var body reauthOnlyRequest
	if !decodeJSONBody(w, r, &body, defaultJSONBodyLimit) {
		return
	}
	if _, ok := h.reauth.Require(w, r, body.CurrentPassword); !ok {
		return
	}
	pub, plain, err := h.apiTokens.Rotate(id)
	if err != nil {
		writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
			Actor:        actorFromRequest(r),
			Action:       "auth.token.rotate",
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
		Action:       "auth.token.rotate",
		ResourceType: "api_token",
		ResourceID:   pub.ID,
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
		Details:      map[string]any{"name": pub.Name, "scope": pub.Scope},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"token":  pub,
		"secret": plain,
	})
}

func (h *APITokensHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.apiTokens == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "api tokens not configured"})
		return
	}
	id := r.PathValue("id")
	var body reauthOnlyRequest
	if !decodeJSONBody(w, r, &body, defaultJSONBodyLimit) {
		return
	}
	if _, ok := h.reauth.Require(w, r, body.CurrentPassword); !ok {
		return
	}
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

func parseOptionalExpiry(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, errors.New("expires_at must be RFC3339")
	}
	return ts, nil
}

func writeTokenStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidTokenName), errors.Is(err, auth.ErrInvalidScope), errors.Is(err, auth.ErrInvalidExpiry):
		writeBadRequest(w, err.Error())
	case errors.Is(err, auth.ErrTokenNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
	case errors.Is(err, auth.ErrTokenLimit):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
	default:
		writeInternalError(w, "api tokens", err)
	}
}
