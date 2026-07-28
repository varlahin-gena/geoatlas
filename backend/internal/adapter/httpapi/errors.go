package httpapi

import (
	"errors"
	"net/http"

	"network_monitor/internal/apperr"
	"network_monitor/internal/usecase/parseerrors"
	usecaseretention "network_monitor/internal/usecase/retention"
)

// writeDomainError maps application sentinel errors to HTTP status.
// Unknown errors go through writeInternalError (logged 500).
func writeDomainError(w http.ResponseWriter, logMsg string, err error) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
	case errors.Is(err, apperr.ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
	case apperr.IsClient(err),
		errors.Is(err, parseerrors.ErrNoIDs),
		errors.Is(err, usecaseretention.ErrInvalidDays):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	default:
		writeInternalError(w, logMsg, err)
	}
}
