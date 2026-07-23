package clickhouse

import (
	"context"
	"strings"
	"testing"

	"network_monitor/internal/model"
)

func TestLoadGeoRangesRejectsNil(t *testing.T) {
	_, err := LoadGeoRanges(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("nil conn: %v", err)
	}
}

func TestReplaceGeoRangesRejectsEmpty(t *testing.T) {
	_, err := ReplaceGeoRanges(context.Background(), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "no geo ranges") {
		t.Fatalf("empty ranges: %v", err)
	}
	_, err = ReplaceGeoRanges(context.Background(), nil, []model.GeoRange{{StartIP: 1, EndIP: 2}})
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("nil conn: %v", err)
	}
}
