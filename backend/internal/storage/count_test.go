package storage

import (
	"context"
	"testing"
)

func TestCountTableRowsAllowlist(t *testing.T) {
	_, err := CountTableRows(context.Background(), nil, "evil")
	if err == nil {
		t.Fatal("expected reject for unknown table")
	}
	_, err = CountTableRows(context.Background(), nil, "traffic_logs")
	if err == nil {
		t.Fatal("expected error for nil conn")
	}
}
