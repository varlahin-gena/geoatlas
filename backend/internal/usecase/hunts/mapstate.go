package hunts

import (
	"strconv"
	"strings"
	"time"

	"geoatlas/internal/mapsearch"
	"geoatlas/internal/model"
)

func mapStateToTimeRange(st MapState, now time.Time) model.TimeRange {
	now = now.UTC()
	if strings.TrimSpace(st.Period) == "custom" {
		from, _ := parseTime(st.PeriodFrom)
		to, _ := parseTime(st.PeriodTo)
		if to.IsZero() {
			to = now
		}
		if from.IsZero() {
			from = to.Add(-24 * time.Hour)
		}
		if to.Before(from) {
			from, to = to, from
		}
		const maxRange = 30 * 24 * time.Hour
		if to.Sub(from) > maxRange {
			from = to.Add(-maxRange)
		}
		return model.TimeRange{Mode: "absolute", From: from, To: to}
	}
	p := strings.TrimSpace(st.Period)
	if p == "" {
		p = "1d"
	}
	if strings.HasSuffix(p, "m") {
		n, _ := strconv.Atoi(strings.TrimSuffix(p, "m"))
		if n < 1 {
			n = 15
		}
		if n > 30*24*60 {
			n = 30 * 24 * 60
		}
		return model.TimeRange{Mode: "minutes", Amount: n}
	}
	if strings.HasSuffix(p, "h") {
		n, _ := strconv.Atoi(strings.TrimSuffix(p, "h"))
		if n < 1 {
			n = 1
		}
		if n > 30*24 {
			n = 30 * 24
		}
		return model.TimeRange{Mode: "hours", Amount: n}
	}
	if strings.HasSuffix(p, "d") {
		n, _ := strconv.Atoi(strings.TrimSuffix(p, "d"))
		if n < 1 {
			n = 1
		}
		if n > 30 {
			n = 30
		}
		return model.TimeRange{Mode: "days", Amount: n}
	}
	return model.TimeRange{Mode: "days", Amount: 1}
}

func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
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

func normalizeGroupBy(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "ip", "subnet", "country", "city", "continent":
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

func mapLimit(st MapState) int {
	if st.Limit > 0 {
		return st.Limit
	}
	return 5000
}

func mapCostInput(st MapState, tr model.TimeRange) mapsearch.MapQueryCostInput {
	from, to := tr.From, tr.To
	switch tr.Mode {
	case "minutes":
		to = time.Now().UTC()
		from = to.Add(-time.Duration(tr.Amount) * time.Minute)
	case "hours":
		to = time.Now().UTC()
		from = to.Add(-time.Duration(tr.Amount) * time.Hour)
	case "days":
		to = time.Now().UTC()
		from = to.Add(-time.Duration(tr.Amount) * 24 * time.Hour)
	}
	return mapsearch.MapQueryCostInput{
		GroupBy: normalizeGroupBy(st.GroupBy),
		Mode:    tr.Mode,
		Amount:  tr.Amount,
		From:    from,
		To:      to,
		Country: strings.TrimSpace(st.Country),
		Query:   strings.TrimSpace(st.Query),
	}
}
