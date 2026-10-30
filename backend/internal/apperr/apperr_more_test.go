package apperr_test

import (
	"errors"
	"testing"

	"geoatlas/internal/apperr"
)

func TestEmptyMessagesAndTooLarge(t *testing.T) {
	if !errors.Is(apperr.InvalidInput(""), apperr.ErrInvalidInput) {
		t.Fatal("empty invalid")
	}
	if !errors.Is(apperr.NotFound(""), apperr.ErrNotFound) {
		t.Fatal("empty notfound")
	}
	if !errors.Is(apperr.Conflict(""), apperr.ErrConflict) {
		t.Fatal("empty conflict")
	}
	if !errors.Is(apperr.TooLarge(""), apperr.ErrTooLarge) {
		t.Fatal("empty too large")
	}
	if !errors.Is(apperr.TooLarge("payload"), apperr.ErrTooLarge) {
		t.Fatal("msg too large")
	}
	if apperr.InvalidCSV(nil) != nil {
		t.Fatal("nil csv")
	}
	if apperr.IsClient(nil) || apperr.IsClient(apperr.TooLarge("x")) {
		t.Fatal("IsClient")
	}
}
