package parsetest

import (
	"network_monitor/internal/model"
)

// ParseResult — итог разбора одной строки (без зависимости от parser package).
type ParseResult struct {
	OK      bool
	Skipped bool
	Log     model.TrafficLog
	Vendor  string
	Reason  string
}

// VerboseParser — подробный разбор для тест-страницы.
type VerboseParser interface {
	ParseVerbose(line string) ParseResult
}

// GeoLookuper — live Lookup для обогащения country в результатах теста.
type GeoLookuper interface {
	Lookup(ipStr string) model.GeoLookup
}

// SamplesProvider — пресеты строк по вендорам.
type SamplesProvider interface {
	SamplesByVendor() map[string][]string
}
