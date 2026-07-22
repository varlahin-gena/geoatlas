package httpapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	usecasegeo "network_monitor/internal/usecase/geo"
)

func (h *GeoHandler) UploadGeo(w http.ResponseWriter, r *http.Request) {
	if h.geoUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "geo service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	dryRun := isDryRun(r)

	var reader io.Reader
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		file, _, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing file"})
			return
		}
		defer file.Close()
		reader = file
	} else {
		if r.Body == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "empty body"})
			return
		}
		defer r.Body.Close()
		reader = r.Body
	}

	result, err := h.geoUC.UploadCSV(ctx, reader, dryRun)
	if err != nil {
		if usecasegeo.IsClientCSVError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeInternalError(w, "geo csv upload failed", err)
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
