package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"network_monitor/internal/model"
	usecaseevents "network_monitor/internal/usecase/events"
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

func (h *EventsHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	if h.eventsUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "events service unavailable"})
		return
	}
	q := r.URL.Query()
	tr, err := parseEventTimeRange(q)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid time range"})
		return
	}

	result, err := h.eventsUC.GetMap(r.Context(), usecaseevents.GetMapInput{
		TimeRange: model.TimeRange{Mode: tr.Mode, Amount: tr.Amount, From: tr.From, To: tr.To},
		Limit:     parseOptionalLimit(q.Get("limit")),
		GroupBy:   normalizeGroupBy(q.Get("group_by")),
		Filter:    normalizeFilter(q.Get("filter")),
		Timeout:   h.cfg.QueryTimeout,
	})
	if err != nil {
		writeInternalError(w, "events: get map failed", err)
		return
	}

	resp := map[string]any{
		"group_by": result.GroupBy,
		"filter":   result.Filter,
		"period":   result.Period,
		"lines":    result.Lines,
		"points":   result.Points,
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
