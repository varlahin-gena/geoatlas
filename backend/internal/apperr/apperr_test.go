package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"geoatlas/internal/apperr"
)

func TestMarkedPreservesMessageAndKind(t *testing.T) {
	err := apperr.InvalidInput("invalid IPv4 address")
	if err.Error() != "invalid IPv4 address" {
		t.Fatalf("Error()=%q", err.Error())
	}
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatal("expected ErrInvalidInput")
	}
	if apperr.IsClient(err) != true {
		t.Fatal("expected IsClient")
	}
}

func TestConflictAndNotFound(t *testing.T) {
	if !errors.Is(apperr.Conflict("overlap"), apperr.ErrConflict) {
		t.Fatal("conflict")
	}
	if !errors.Is(apperr.NotFound("missing"), apperr.ErrNotFound) {
		t.Fatal("not found")
	}
	if apperr.IsClient(apperr.Conflict("x")) {
		t.Fatal("conflict is not a plain client/400")
	}
}

func TestInvalidCSV(t *testing.T) {
	inner := errors.New("missing required columns: Network")
	err := apperr.InvalidCSV(inner)
	if err.Error() != inner.Error() {
		t.Fatalf("msg=%q", err.Error())
	}
	if !errors.Is(err, apperr.ErrInvalidCSV) || !apperr.IsClient(err) {
		t.Fatal("csv kind")
	}
	// idempotent
	if !errors.Is(apperr.InvalidCSV(err), apperr.ErrInvalidCSV) {
		t.Fatal("rewrap")
	}
}

func TestWrappedStillMatches(t *testing.T) {
	err := fmt.Errorf("upload: %w", apperr.InvalidInput("bad"))
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatal("wrapped")
	}
}
