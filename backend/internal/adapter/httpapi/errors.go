package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"network_monitor/internal/apperr"
	usecasebackup "network_monitor/internal/usecase/backup"
)

// writeDomainError maps application sentinel errors to HTTP status.
// Unknown errors go through writeInternalError (logged 500).
func writeDomainError(w http.ResponseWriter, logMsg string, err error) {
	if err == nil {
		return
	}
	var maxBytes *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytes):
		msg := err.Error()
		if maxBytes != nil && maxBytes.Limit > 0 {
			msg = "request body too large (limit " + strconv.FormatInt(maxBytes.Limit, 10) +
				" bytes; GEOIP_UPLOAD_MAX_BYTES / MAX_GEO_UPLOAD_SIZE)"
		}
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": msg})
	case errors.Is(err, apperr.ErrTooLarge):
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": err.Error()})
	case errors.Is(err, apperr.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
	case errors.Is(err, apperr.ErrConflict):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
	case errors.Is(err, usecasebackup.ErrDisabled),
		errors.Is(err, usecasebackup.ErrUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
	case apperr.IsClient(err):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
	default:
		writeInternalError(w, logMsg, err)
	}
}
