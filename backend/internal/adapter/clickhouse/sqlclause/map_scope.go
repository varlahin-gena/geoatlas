package sqlclause

import (
	"strings"
	"unicode/utf8"
)

const (
	maxMapCountryRunes = 80
	maxMapQueryRunes   = 120
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

func (s MapScope) Empty() bool {
	c := s.sanitized()
	return c.Country == "" && c.Query == ""
}

// LogsWhere — AND-фрагмент для traffic_logs (до GROUP BY). args — bind-параметры.
func (s MapScope) LogsWhere() (clause string, args []any) {
	c := s.sanitized()
	if c.Country != "" {
		clause += " AND (lowerUTF8(src_country) = lowerUTF8(?) OR lowerUTF8(dst_country) = lowerUTF8(?))"
		args = append(args, c.Country, c.Country)
	}
	if c.Query != "" {
		clause += ` AND positionCaseInsensitiveUTF8(concat_ws(' ', toString(src_ip), toString(dst_ip), src_country, dst_country, src_city, dst_city, rule, device, proto, src_zone, dst_zone), ?) > 0`
		args = append(args, c.Query)
	}
	return clause, args
}

// GeoAggHavingExpr — условие HAVING для traffic_edges_{city|country}_daily
// (после GROUP BY доступны алиасы src_key/src_country/…).
func (s MapScope) GeoAggHavingExpr() (expr string, args []any) {
	c := s.sanitized()
	var parts []string
	if c.Country != "" {
		parts = append(parts, "(lowerUTF8(src_country) = lowerUTF8(?) OR lowerUTF8(dst_country) = lowerUTF8(?) OR lowerUTF8(src_key) = lowerUTF8(?) OR lowerUTF8(dst_key) = lowerUTF8(?))")
		args = append(args, c.Country, c.Country, c.Country, c.Country)
	}
	if c.Query != "" {
		parts = append(parts, `positionCaseInsensitiveUTF8(concat_ws(' ', src_key, dst_key, src_label, dst_label, src_country, dst_country, src_city, dst_city, rule, device, proto), ?) > 0`)
		args = append(args, c.Query)
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
