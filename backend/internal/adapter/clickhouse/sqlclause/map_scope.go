package sqlclause

import (
	"strings"
	"unicode/utf8"

	"network_monitor/internal/mapsearch"
)

const (
	maxMapCountryRunes = 80
	maxMapQueryRunes   = 400
)

// MapScope — country/q для карты. Значения только как bind-args, не в SQL-текст.
type MapScope struct {
	Country string
	Query   string
}

func clipRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || max < 1 {
		return ""
	}
	s = strings.ReplaceAll(s, "\x00", "")
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

func SanitizeMapCountry(s string) string { return clipRunes(s, maxMapCountryRunes) }
func SanitizeMapQuery(s string) string   { return clipRunes(s, maxMapQueryRunes) }

func (s MapScope) sanitized() MapScope {
	return MapScope{Country: SanitizeMapCountry(s.Country), Query: SanitizeMapQuery(s.Query)}
}

func countryPred() string {
	return "(lowerUTF8(src_country) = lowerUTF8(?) OR lowerUTF8(dst_country) = lowerUTF8(?))"
}

// LogsWhere — AND-фрагмент для traffic_logs (до GROUP BY). args — bind-параметры.
func (s MapScope) LogsWhere() (clause string, args []any) {
	c := s.sanitized()
	if c.Country != "" {
		clause += " AND " + countryPred()
		args = append(args, c.Country, c.Country)
	}
	if c.Query != "" {
		pred, sargs := MapSearchSQL(mapsearch.Compile(c.Query), LogsMapSearchColumns)
		if pred != "" {
			clause += " AND (" + pred + ")"
			args = append(args, sargs...)
		}
	}
	return clause, args
}

// GeoAggHavingExpr — условие HAVING для traffic_edges_{city|country}_daily
func (s MapScope) GeoAggHavingExpr() (expr string, args []any) {
	c := s.sanitized()
	var parts []string
	if c.Country != "" {
		parts = append(parts, "(lowerUTF8(src_country) = lowerUTF8(?) OR lowerUTF8(dst_country) = lowerUTF8(?) OR lowerUTF8(src_key) = lowerUTF8(?) OR lowerUTF8(dst_key) = lowerUTF8(?))")
		args = append(args, c.Country, c.Country, c.Country, c.Country)
	}
	if c.Query != "" {
		pred, sargs := MapSearchSQL(mapsearch.Compile(c.Query), GeoMapSearchColumns)
		if pred != "" {
			parts = append(parts, "("+pred+")")
			args = append(args, sargs...)
		}
	}
	return strings.Join(parts, " AND "), args
}

// IPAggHavingExpr — HAVING для traffic_edges_daily/hourly (GROUP BY src_ip, dst_ip).
func (s MapScope) IPAggHavingExpr() (expr string, args []any) {
	c := s.sanitized()
	var parts []string
	if c.Country != "" {
		parts = append(parts, countryPred())
		args = append(args, c.Country, c.Country)
	}
	if c.Query != "" {
		pred, sargs := MapSearchSQL(mapsearch.Compile(c.Query), IPAggMapSearchColumns)
		if pred != "" {
			parts = append(parts, "("+pred+")")
			args = append(args, sargs...)
		}
	}
	return strings.Join(parts, " AND "), args
}

// JoinHaving склеивает HAVING-фрагменты (` HAVING x` или голое выражение).
func JoinHaving(parts ...string) string {
	var exprs []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, "HAVING")
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, "AND")
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		exprs = append(exprs, p)
	}
	if len(exprs) == 0 {
		return ""
	}
	return " HAVING " + strings.Join(exprs, " AND ")
}
