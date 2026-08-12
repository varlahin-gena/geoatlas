package clickhouse

import (
	"context"
	"log/slog"
	"runtime"
	"sync/atomic"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/geoip"
)

// ReloadableGeoIndex — *geoip.Index + Reload из ClickHouse.
type ReloadableGeoIndex struct {
	*geoip.Index
	ch    clickhouse.Conn
	ready atomic.Bool
}

func NewReloadableGeoIndex(ch clickhouse.Conn) *ReloadableGeoIndex {
	return &ReloadableGeoIndex{Index: geoip.New(), ch: ch}
}

// IndexReady — true после первой попытки Reload (успех или пустая база).
// Пока false, in-memory индекс ещё поднимается асинхронно при старте.
func (i *ReloadableGeoIndex) IndexReady() bool {
	return i != nil && i.ready.Load()
}

func (i *ReloadableGeoIndex) Reload(ctx context.Context) error {
	if i == nil || i.Index == nil {
		return nil
	}
	defer i.ready.Store(true)

	before := i.RangeCount()
	var msBefore runtime.MemStats
	runtime.ReadMemStats(&msBefore)

	built, err := LoadGeoSnapshot(ctx, i.ch)
	if err != nil {
		return err
	}
	skipped := 0
	if built != nil {
		skipped = built.Skipped()
	}
	if skipped > 0 {
		slog.Warn("geo index: overlapping or invalid ranges skipped", "skipped", skipped, "kept", built.RangeCount())
	}
	i.ReplaceBuiltSnapshot(built)

	var msAfter runtime.MemStats
	runtime.ReadMemStats(&msAfter)
	slog.Info("geo index loaded",
		"ranges", built.RangeCount(),
		"prev_ranges", before,
		"index_bytes_mb", float64(i.ApproxBytes())/(1<<20),
		"heap_alloc_mb", msAfter.Alloc/(1<<20),
		"heap_sys_mb", msAfter.Sys/(1<<20),
		"heap_delta_mb", int64(msAfter.Alloc-msBefore.Alloc)/(1<<20),
	)
	return nil
}
