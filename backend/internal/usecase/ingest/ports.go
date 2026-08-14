package ingest

import (
	"context"
	"time"

	"network_monitor/internal/model"
	"network_monitor/internal/parser"
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

// LineParser — разбор syslog/лог-строк.
type LineParser interface {
	ParseVerbose(line string) parser.ParseResult
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
	// AddBufferDropped — потери из processor-буфера (не путать с queue DroppedTotal).
	AddBufferDropped(n int64)
	SetLastFlushAt(t time.Time)
}

// InsertObserver — latency/outcome батчевых INSERT (опционально; Prometheus).
type InsertObserver interface {
	ObserveInsert(d time.Duration, rows int, success bool)
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
	// Circuit — общий breaker для workers; nil → NewProcessor создаёт приватный.
	Circuit *CircuitBreaker
	// InsertObs — опциональные метрики insert; nil = no-op.
	InsertObs InsertObserver
}
