package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"network_monitor/internal/model"
)

var errIngestUnavailable = errors.New("ingest unavailable")

func (h *IngestHandler) ingestReader(ctx context.Context, r io.Reader) (model.IngestStats, error) {
	if h.ingest == nil {
		return model.IngestStats{}, errIngestUnavailable
	}
	return h.ingest.FeedReader(ctx, r, "http")
}

func (h *IngestHandler) GetIngestStats(w http.ResponseWriter, r *http.Request) {
	if h.ingest == nil {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false})
		return
	}
	writeJSON(w, http.StatusOK, h.ingest.Stats())
}

func (h *IngestHandler) IngestLogs(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	stats, err := h.ingestReader(r.Context(), r.Body)
	if errors.Is(err, errIngestUnavailable) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "ingest unavailable", "stats": stats})
		return
	}
	if err != nil {
		slog.Error("ingest logs failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal server error", "stats": stats})
		return
	}
	writeIngestAccepted(w, h.ingestRetryAfterSec(), stats)
}

func (h *IngestHandler) UploadLogs(w http.ResponseWriter, r *http.Request) {
	var reader io.Reader

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		file, _, err := r.FormFile("file")
		if err != nil {
			writeBadRequest(w, "missing file")
			return
		}
		defer file.Close()
		reader = file
	} else {
		if r.Body == nil {
			writeBadRequest(w, "empty body")
			return
		}
		defer r.Body.Close()
		reader = r.Body
	}

	stats, err := h.ingestReader(r.Context(), reader)
	if errors.Is(err, errIngestUnavailable) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "ingest unavailable", "stats": stats})
		return
	}
	if err != nil {
		slog.Error("upload logs failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal server error", "stats": stats})
		return
	}
	writeIngestAccepted(w, h.ingestRetryAfterSec(), stats)
}

func (h *IngestHandler) ingestRetryAfterSec() int {
	sec := 3
	if h != nil && h.cfg.IngestFlushSec > 0 {
		sec = h.cfg.IngestFlushSec
	}
	return sec
}

// writeIngestAccepted — HTTP delivery contract:
//   - 200, если все строки приняты в очередь (dropped=0);
//   - 503 + Retry-After, если очередь была полна и часть/все строки дропнуты.
func writeIngestAccepted(w http.ResponseWriter, retryAfterSec int, stats model.IngestStats) {
	if stats.Dropped > 0 {
		if retryAfterSec < 1 {
			retryAfterSec = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSec))
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok":    false,
			"error": "ingest queue full — lines dropped; retry after backoff",
			"stats": stats,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "stats": stats})
}
