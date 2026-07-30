package bootstrap

import (
	"context"
	"time"
)

// SchemaEnsurer — Ensure* DDL/MV при старте.
type SchemaEnsurer interface {
	EnsureTTLOnlyDropParts(ctx context.Context) error
	EnsureTrafficLogsSuccess(ctx context.Context) error
	EnsureEdgesAggSchema(ctx context.Context) error
	EnsureGeoEdgesAggSchema(ctx context.Context) error
	EnsureReputationRanges(ctx context.Context) error
}

// AggBackfiller — backfill агрегатов.
type AggBackfiller interface {
	BackfillEdgesAgg(ctx context.Context) error
	BackfillGeoEdgesAgg(ctx context.Context) error
}

// AggReadyRefresher — проверка готовности без полного backfill.
type AggReadyRefresher interface {
	RefreshEdgesAggReady(ctx context.Context) error
	RefreshGeoEdgesAggReady(ctx context.Context) error
}

// EnrichScheduler — фоновый geo enrich после Ensure*.
type EnrichScheduler interface {
	ScheduleEnrichOnly(parent context.Context, timeout time.Duration)
}

// GeoRangeCounter — число диапазонов в live GeoIP-индексе.
type GeoRangeCounter interface {
	RangeCount() int
}

// RetentionApplier — повторное применение TTL из файла к ClickHouse.
type RetentionApplier interface {
	ApplyFromStore(ctx context.Context) error
}

// Dependencies — порты для RunStartup.
type Dependencies struct {
	Schema    SchemaEnsurer
	Backfill  AggBackfiller
	Ready     AggReadyRefresher
	Enrich    EnrichScheduler
	Geo       GeoRangeCounter
	Retention RetentionApplier
}

// Options — флаги старта.
type Options struct {
	SkipStartupBackfill     bool
	GeoBackfillLookbackDays int
	// ReputationEnabled — создавать/проверять schema reputation_ranges.
	// false = модуль выключен (REPUTATION_FETCH_ENABLED=false).
	ReputationEnabled bool
	Timeout           time.Duration
}

// WarnFunc логирует нефатальные ошибки Ensure*/backfill.
type WarnFunc func(msg string, err error)

// InfoFunc логирует информационные сообщения.
type InfoFunc func(msg string, args ...any)
