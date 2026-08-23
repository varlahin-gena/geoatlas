package mapsearch

import (
	"strings"
	"unicode/utf8"
)

const (
	MaxMapCountryRunes = 80
	MaxMapQueryRunes   = 400
)

// ClipRunes trim + strip NUL + limit rune length (map country/q).
func ClipRunes(s string, max int) string {
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

func SanitizeMapCountry(s string) string { return ClipRunes(s, MaxMapCountryRunes) }
func SanitizeMapQuery(s string) string   { return ClipRunes(s, MaxMapQueryRunes) }
