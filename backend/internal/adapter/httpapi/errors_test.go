package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"network_monitor/internal/apperr"
	"network_monitor/internal/usecase/parseerrors"
	usecaseretention "network_monitor/internal/usecase/retention"
	"network_monitor/internal/usecase/searchtemplates"
)

func TestWriteDomainErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{"invalid input", apperr.InvalidInput("bad ip"), http.StatusBadRequest, "bad ip"},
		{"invalid csv", apperr.InvalidCSV(errors.New("missing required columns")), http.StatusBadRequest, "missing required columns"},
		{"not found", apperr.NotFound("gone"), http.StatusNotFound, "gone"},
		{"conflict", apperr.Conflict("overlap"), http.StatusConflict, "overlap"},
		{"too large", apperr.TooLarge("geo too big"), http.StatusRequestEntityTooLarge, "geo too big"},
		{"parse ids", parseerrors.ErrNoIDs, http.StatusBadRequest, parseerrors.ErrNoIDs.Error()},
		{"retention", usecaseretention.ErrInvalidDays, http.StatusBadRequest, usecaseretention.ErrInvalidDays.Error()},
		{"templates unavailable", searchtemplates.ErrUnavailable, http.StatusServiceUnavailable, "search templates not configured"},
		{"unknown", errors.New("clickhouse down"), http.StatusInternalServerError, "internal server error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeDomainError(rec, "test", tc.err)
			if rec.Code != tc.status {
				t.Fatalf("status=%d want %d body=%s", rec.Code, tc.status, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.body) {
				t.Fatalf("body=%s want contains %q", rec.Body.String(), tc.body)
			}
		})
	}
}
