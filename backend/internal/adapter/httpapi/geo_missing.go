package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"network_monitor/internal/model"
	usecasegeo "network_monitor/internal/usecase/geo"
)

func parseGeoMissingLimit(v string) int {
	limit := 500
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
		limit = n
	}
	if limit > 5000 {
		limit = 5000
	}
	return limit
}

// GetGeoMissing — IP из трафика за период, которые нельзя поставить на карту (нет координат).
func (h *GeoHandler) GetGeoMissing(w http.ResponseWriter, r *http.Request) {
	if h.geoUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "geo service unavailable"})
		return
	}
	tr, err := parseEventTimeRange(r.URL.Query())
	if err != nil {
		writeBadRequest(w, "invalid time range")
		return
	}

	result, err := h.geoUC.ListMissing(r.Context(), usecasegeo.ListMissingInput{
		TimeRange: model.TimeRange{Mode: tr.Mode, Amount: tr.Amount, From: tr.From, To: tr.To},
		Limit:     parseGeoMissingLimit(r.URL.Query().Get("limit")),
		Timeout:   h.cfg.QueryTimeout,
	})
	if err != nil {
		writeInternalError(w, "geo-missing: list failed", err)
		return
	}

	items := make([]map[string]any, 0, len(result.Items))
	for _, it := range result.Items {
		items = append(items, map[string]any{
			"ip": it.IP, "kind": it.Kind, "count": it.Count,
			"as_src": it.AsSrc, "as_dst": it.AsDst,
			"sample_peer": it.SamplePeer, "log_country": it.LogCountry,
			"log_city": it.LogCity, "action_hint": it.ActionHint,
		})
	}

	resp := map[string]any{
		"items": items, "summary": result.Summary,
		"period": result.Period, "limit": result.Limit,
	}
	switch result.Period {
	case "absolute":
		resp["from"] = result.From.Format(time.RFC3339)
		resp["to"] = result.To.Format(time.RFC3339)
	case "minutes", "hours", "days":
		resp[result.Period] = result.Amount
	}
	writeJSON(w, http.StatusOK, resp)
}
