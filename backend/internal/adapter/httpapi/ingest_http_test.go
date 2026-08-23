package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"geoatlas/internal/model"
)

func TestWriteIngestAcceptedOK(t *testing.T) {
	rec := httptest.NewRecorder()
	writeIngestAccepted(rec, 3, model.IngestStats{Received: 10, Queued: 10})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != true {
		t.Fatalf("body=%v", body)
	}
}

func TestWriteIngestAcceptedQueueFull(t *testing.T) {
	rec := httptest.NewRecorder()
	writeIngestAccepted(rec, 5, model.IngestStats{Received: 10, Queued: 2, Dropped: 8})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("Retry-After=%q want 5", got)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != false {
		t.Fatalf("ok=%v", body["ok"])
	}
	stats, _ := body["stats"].(map[string]any)
	if stats["dropped"] != float64(8) {
		t.Fatalf("stats=%v", stats)
	}
}

func TestWriteIngestAcceptedAllDropped(t *testing.T) {
	rec := httptest.NewRecorder()
	writeIngestAccepted(rec, 0, model.IngestStats{Received: 3, Dropped: 3})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After=%q want 1 (floor)", got)
	}
}
