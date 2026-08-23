package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"geoatlas/internal/model"
	usecaseevents "geoatlas/internal/usecase/events"
)

func parseDays(v string) int {
	days := 1
	if n, err := strconv.Atoi(v); err == nil {
		days = n
	}
	if days < 1 {
		days = 1
	}
	if days > 30 {
		days = 30
	}
	return days
}

type eventTimeRange struct {
	Mode   string
	Amount int
	From   time.Time
	To     time.Time
}

func parseTimeParam(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, strconv.ErrSyntax
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, strconv.ErrSyntax
}

func parseEventTimeRange(q map[string][]string) (eventTimeRange, error) {
	get := func(key string) string {
		if vals, ok := q[key]; ok && len(vals) > 0 {
			return strings.TrimSpace(vals[0])
		}
		return ""
	}

	now := time.Now().UTC()

	if fromStr := get("from"); fromStr != "" {
		from, err := parseTimeParam(fromStr)
		if err != nil {
			return eventTimeRange{}, err
		}
		to := now
		if toStr := get("to"); toStr != "" {
			to, err = parseTimeParam(toStr)
			if err != nil {
				return eventTimeRange{}, err
			}
		}
		if to.Before(from) {
			from, to = to, from
		}
		const maxRange = 30 * 24 * time.Hour
		if to.Sub(from) > maxRange {
			from = to.Add(-maxRange)
		}
		return eventTimeRange{Mode: "absolute", From: from.UTC(), To: to.UTC()}, nil
	}

	if mStr := get("minutes"); mStr != "" {
		m, err := strconv.Atoi(mStr)
		if err != nil || m < 1 {
			m = 15
		}
		if m > 30*24*60 {
			m = 30 * 24 * 60
		}
		return eventTimeRange{Mode: "minutes", Amount: m}, nil
	}

	if hStr := get("hours"); hStr != "" {
		h, err := strconv.Atoi(hStr)
		if err != nil || h < 1 {
			h = 1
		}
		if h > 30*24 {
			h = 30 * 24
		}
		return eventTimeRange{Mode: "hours", Amount: h}, nil
	}

	days := parseDays(get("days"))
	return eventTimeRange{Mode: "days", Amount: days}, nil
}

const (
	defaultEventsLimit = 10000
	maxEventsLimit     = 50000
)

// parseOptionalLimit возвращает лимит сырых пар для /api/events.
func parseOptionalLimit(v string) int {
	if v == "" {
		return defaultEventsLimit
	}
	limit, err := strconv.Atoi(v)
	if err != nil || limit <= 0 {
		return defaultEventsLimit
	}
	if limit > maxEventsLimit {
		limit = maxEventsLimit
	}
	return limit
}

func normalizeGroupBy(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "ip", "subnet", "country", "city":
		return v
	default:
		return "city"
	}
}

func normalizeFilter(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "allowed", "blocked":
		return v
	default:
		return "all"
	}
}

func normalizeRepSide(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "src", "dst", "both":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "any"
	}
}

func parseCSVParam(v string, max int) []string {
	if v == "" || max < 1 {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len([]rune(p)) > 64 {
			p = string([]rune(p)[:64])
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
		if len(out) >= max {
			break
		}
	}
	return out
}

func normalizeDataSource(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "backup" {
		return "backup"
	}
	return "live"
}

func (h *EventsHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	if h.eventsUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "events service unavailable"})
		return
	}
	q := r.URL.Query()
	tr, err := parseEventTimeRange(q)
	if err != nil {
		writeBadRequest(w, "invalid time range")
		return
	}

	dataSource := normalizeDataSource(q.Get("source"))
	attached := ""
	if h.backupUC != nil {
		attached = h.backupUC.AttachedName()
	}
	if dataSource == "backup" && attached == "" {
		writeBadRequest(w, "backup not attached; connect a backup in System → Резервное копирование")
		return
	}

	result, err := h.eventsUC.GetMap(r.Context(), usecaseevents.GetMapInput{
		TimeRange:     model.TimeRange{Mode: tr.Mode, Amount: tr.Amount, From: tr.From, To: tr.To},
		Limit:         parseOptionalLimit(q.Get("limit")),
		GroupBy:       normalizeGroupBy(q.Get("group_by")),
		Filter:        normalizeFilter(q.Get("filter")),
		Country:       q.Get("country"),
		Query:         q.Get("q"),
		RepCategories: parseCSVParam(q.Get("rep_cat"), 32),
		RepLists:      parseCSVParam(q.Get("rep_list"), 32),
		RepSide:       normalizeRepSide(q.Get("rep_side")),
		DataSource:    dataSource,
		Timeout:       h.cfg.QueryTimeout,
	})
	if err != nil {
		writeInternalError(w, "events: get map failed", err)
		return
	}

	resp := map[string]any{
		"group_by":          result.GroupBy,
		"filter":            result.Filter,
		"country":           result.Country,
		"q":                 result.Query,
		"period":            result.Period,
		"data_source":       dataSource,
		"backup_attached":   attached,
		"lines":             result.Lines,
		"points":            result.Points,
		"reputation_facets": result.ReputationFacets,
		"stats": map[string]any{
			"raw_pairs":      result.RawPairs,
			"edges":          len(result.Lines),
			"nodes":          len(result.Points),
			"skipped_no_geo": result.SkippedNoGeo,
			"source":         result.Source,
		},
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

func (h *EventsHandler) GetEventsSeries(w http.ResponseWriter, r *http.Request) {
	if h.eventsUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "events service unavailable"})
		return
	}
	q := r.URL.Query()
	country := strings.TrimSpace(q.Get("country"))
	if country == "" {
		writeBadRequest(w, "country is required")
		return
	}
	tr, err := parseEventTimeRange(q)
	if err != nil {
		writeBadRequest(w, "invalid time range")
		return
	}
	dataSource := normalizeDataSource(q.Get("source"))
	attached := ""
	if h.backupUC != nil {
		attached = h.backupUC.AttachedName()
	}
	if dataSource == "backup" && attached == "" {
		writeBadRequest(w, "backup not attached; connect a backup in System → Резервное копирование")
		return
	}
	result, err := h.eventsUC.GetSeries(r.Context(), usecaseevents.GetSeriesInput{
		TimeRange:  model.TimeRange{Mode: tr.Mode, Amount: tr.Amount, From: tr.From, To: tr.To},
		Country:    country,
		DataSource: dataSource,
		Timeout:    h.cfg.QueryTimeout,
	})
	if err != nil {
		writeInternalError(w, "events: get series failed", err)
		return
	}
	points := make([]map[string]any, 0, len(result.Points))
	for _, p := range result.Points {
		points = append(points, map[string]any{
			"t":       p.T.UTC().Format(time.RFC3339),
			"allowed": p.Allowed,
			"blocked": p.Blocked,
			"total":   p.Total,
		})
	}
	resp := map[string]any{
		"country":    result.Country,
		"bucket_sec": result.BucketSec,
		"period":     result.Period,
		"points":     points,
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
