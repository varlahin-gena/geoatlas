package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"network_monitor/internal/apperr"
)

func (h *GeoHandler) UploadGeo(w http.ResponseWriter, r *http.Request) {
	if h.geoUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "geo service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	dryRun := isDryRun(r)
	indexRanges := h.geoUC.IndexRangeCount()
	slog.Info("geo upload start",
		"dry_run", dryRun,
		"content_length", r.ContentLength,
		"index_ranges", indexRanges,
	)

	// Early 409 до чтения body — иначе клиент успеет залить сотни МиБ впустую.
	if err := h.geoUC.PrecheckUpload(dryRun); err != nil {
		slog.Info("geo upload early reject", "dry_run", dryRun, "index_ranges", indexRanges, "err", err.Error())
		writeDomainError(w, "geo csv upload failed", err)
		return
	}

	if max := h.cfg.MaxGeoUploadSize; max > 0 && r.ContentLength > max {
		err := apperr.TooLarge(
			"request body too large (Content-Length " + strconv.FormatInt(r.ContentLength, 10) +
				", limit " + strconv.FormatInt(max, 10) + " bytes; GEOIP_UPLOAD_MAX_BYTES / MAX_GEO_UPLOAD_SIZE)",
		)
		slog.Info("geo upload early reject", "dry_run", dryRun, "content_length", r.ContentLength, "err", err.Error())
		writeDomainError(w, "geo csv upload failed", err)
		return
	}

	var reader io.Reader
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		file, _, err := r.FormFile("file")
		if err != nil {
			var maxBytes *http.MaxBytesError
			if errors.As(err, &maxBytes) {
				writeDomainError(w, "geo csv upload failed", err)
				return
			}
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

	result, err := h.geoUC.UploadCSV(ctx, reader, dryRun)
	if err != nil {
		writeDomainError(w, "geo csv upload failed", err)
		return
	}

	if result.DryRun {
		sample := make([]map[string]any, 0, len(result.Sample))
		for _, g := range result.Sample {
			sample = append(sample, map[string]any{
				"start_ip": g.StartIP,
				"end_ip":   g.EndIP,
				"country":  g.Country,
				"region":   g.Region,
				"city":     g.City,
				"lat":      g.Lat,
				"lon":      g.Lon,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "dry_run": true, "ranges": result.Count, "sample": sample,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "ranges": result.Count, "reload": result.Reload, "backfill": result.Backfill,
	})
}

func isDryRun(r *http.Request) bool {
	v := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("dry_run")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
