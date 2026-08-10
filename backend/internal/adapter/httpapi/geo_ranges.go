package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"network_monitor/internal/model"
	usecasegeo "network_monitor/internal/usecase/geo"
)

type geoRangeDTO struct {
	Network string  `json:"network"`
	StartIP uint32  `json:"start_ip"`
	EndIP   uint32  `json:"end_ip"`
	Country string  `json:"country"`
	Region  string  `json:"region"`
	City    string  `json:"city"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

type appendGeoRequest struct {
	Network string  `json:"network"`
	Country string  `json:"country"`
	Region  string  `json:"region"`
	City    string  `json:"city"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

type updateGeoRequest struct {
	OriginalNetwork string  `json:"original_network"`
	Network         string  `json:"network"`
	Country         string  `json:"country"`
	Region          string  `json:"region"`
	City            string  `json:"city"`
	Lat             float64 `json:"lat"`
	Lon             float64 `json:"lon"`
}

func (h *GeoHandler) toGeoRangeDTO(g model.GeoRange) geoRangeDTO {
	network := ""
	if h.geoUC != nil {
		network = h.geoUC.FormatNetwork(g.StartIP, g.EndIP)
	}
	return geoRangeDTO{
		Network: network,
		StartIP: g.StartIP,
		EndIP:   g.EndIP,
		Country: g.Country,
		Region:  g.Region,
		City:    g.City,
		Lat:     g.Lat,
		Lon:     g.Lon,
	}
}

func parseGeoRangesLimit(v string) int {
	limit := 2000
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
		limit = n
	}
	if limit > 10000 {
		limit = 10000
	}
	return limit
}

func (h *GeoHandler) ListGeoRanges(w http.ResponseWriter, r *http.Request) {
	if h.geoUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "geo service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	result, err := h.geoUC.ListRanges(ctx, usecasegeo.ListRangesInput{
		Limit: parseGeoRangesLimit(r.URL.Query().Get("limit")),
		Query: r.URL.Query().Get("q"),
		IP:    strings.TrimSpace(r.URL.Query().Get("ip")),
	})
	if err != nil {
		writeDomainError(w, "geo list failed", err)
		return
	}

	items := make([]geoRangeDTO, 0, len(result.Items))
	for _, g := range result.Items {
		items = append(items, h.toGeoRangeDTO(g))
	}

	limits := map[string]any{
		"upload_max_bytes":  h.cfg.MaxGeoUploadSize,
		"upload_max_ranges": h.cfg.MaxGeoUploadRanges,
	}

	if result.IPLookup {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "count": result.Total, "filtered": result.Filtered,
			"shown": len(items), "ip": result.IP, "ip_hit": result.IPHit, "ranges": items,
			"limits": limits,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "count": result.Total, "filtered": result.Filtered,
		"shown": len(items), "truncated": result.Truncated, "ranges": items,
		"limits": limits,
	})
}

func (h *GeoHandler) AppendGeoRange(w http.ResponseWriter, r *http.Request) {
	if h.geoUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "geo service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	var req appendGeoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}

	result, err := h.geoUC.AppendRange(ctx, req.Network, req.Country, req.Region, req.City, req.Lat, req.Lon)
	if err != nil {
		writeDomainError(w, "geo append failed", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "ranges": result.Count, "added": result.Label,
		"entry": h.toGeoRangeDTO(result.Entry), "reload": result.Reload, "backfill": result.Backfill,
	})
}

func (h *GeoHandler) UpdateGeoRange(w http.ResponseWriter, r *http.Request) {
	if h.geoUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "geo service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	var req updateGeoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}

	result, err := h.geoUC.UpdateRange(ctx, req.OriginalNetwork, req.Network, req.Country, req.Region, req.City, req.Lat, req.Lon)
	if err != nil {
		writeDomainError(w, "geo update failed", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "ranges": result.Count, "updated": result.Label,
		"entry": h.toGeoRangeDTO(result.Entry), "reload": result.Reload, "backfill": result.Backfill,
	})
}

func (h *GeoHandler) ExportGeoRangesCSV(w http.ResponseWriter, r *http.Request) {
	if h.geoUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "geo service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	filename := "geoip-" + time.Now().UTC().Format("20060102-150405") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	if err := h.geoUC.ExportCSV(ctx, w); err != nil {
		slog.Error("geo export: write failed", "err", err)
	}
}

func (h *GeoHandler) ClearGeoRanges(w http.ResponseWriter, r *http.Request) {
	if h.geoUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "geo service unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	result, err := h.geoUC.ClearAll(ctx)
	if err != nil {
		writeDomainError(w, "geo clear failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "index_before": result.IndexBefore, "ranges": 0,
	})
}
