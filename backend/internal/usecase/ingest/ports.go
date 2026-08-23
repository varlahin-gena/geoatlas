package ingest

import (
	"context"
	"time"

	"geoatlas/internal/model"
	"geoatlas/internal/parser"
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

// InsertErrorClassifier — ретраибельны ли ошибки INSERT (реализация в ingeststore).
type InsertErrorClassifier interface {
	IsRetryableInsertError(err error) bool
}

// InsertErrorClassifyFunc адаптирует функцию к InsertErrorClassifier.
type InsertErrorClassifyFunc func(error) bool

func (f InsertErrorClassifyFunc) IsRetryableInsertError(err error) bool {
	if f == nil {
		return true
	}
	return f(err)
}

// Deps — зависимости Processor.
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
	// Retryable — классификатор ошибок INSERT; nil → ретраить всё, кроме cancel/circuit.
	Retryable InsertErrorClassifier
}
