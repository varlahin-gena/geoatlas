package ingest

import (
	"context"
	"time"

	"network_monitor/internal/model"
)

// TrafficLogInserter — batch INSERT в traffic_logs.
type TrafficLogInserter interface {
	InsertTrafficLogs(ctx context.Context, logs []model.TrafficLog) error
}

// ParseErrorInserter — batch INSERT в parse_errors.
type ParseErrorInserter interface {
	InsertParseErrors(ctx context.Context, items []model.ParseError) error
}

// GeoLookup заполняет country/city по IP.
type GeoLookup interface {
	Lookup(ipStr string) model.GeoLookup
}

// ParseResult — итог разбора одной строки (без зависимости от parser package).
type ParseResult struct {
	OK      bool
	Skipped bool
	Log     model.TrafficLog
	Vendor  string
	Reason  string
}

// LineParser — разбор syslog/лог-строк.
type LineParser interface {
	ParseVerbose(line string) ParseResult
	ContainsIPv4(line string) bool
}

// LineStats — опциональные счётчики live-ingest (очередь/мониторинг).
type LineStats interface {
	AddReceived(transport string)
	AddSkipped(n int64)
	AddParsed(n int64)
	AddParseErrors(n int64)
	AddInserted(n int64)
	AddBuffered(delta int64)
	SetLastFlushAt(t time.Time)
}

// Deps — зависимости Processor / sync ProcessReader.
type Deps struct {
	Logs          TrafficLogInserter
	Errors        ParseErrorInserter
	Parser        LineParser
	Geo           GeoLookup
	EnrichCountry bool
	BatchSize     int
	QueryTimeout  time.Duration
}
