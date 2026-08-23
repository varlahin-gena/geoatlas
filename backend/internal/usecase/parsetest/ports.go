package parsetest

import (
	"geoatlas/internal/model"
	"geoatlas/internal/parser"
)

// VerboseParser — подробный разбор для тест-страницы.
type VerboseParser interface {
	ParseVerbose(line string) parser.ParseResult
}

// GeoLookuper — live Lookup для обогащения country в результатах теста.
type GeoLookuper interface {
	Lookup(ipStr string) model.GeoLookup
}

// SamplesProvider — пресеты строк по вендорам.
type SamplesProvider interface {
	SamplesByVendor() map[string][]string
}
