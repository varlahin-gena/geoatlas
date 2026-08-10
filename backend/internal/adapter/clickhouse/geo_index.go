package clickhouse

import (
	"context"
	"log/slog"
	"runtime"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/geoip"
)

// ReloadableGeoIndex — *geoip.Index + Reload из ClickHouse.
type ReloadableGeoIndex struct {
	*geoip.Index
	ch clickhouse.Conn
}

func NewReloadableGeoIndex(ch clickhouse.Conn) *ReloadableGeoIndex {
	return &ReloadableGeoIndex{Index: geoip.New(), ch: ch}
}

func (i *ReloadableGeoIndex) Reload(ctx context.Context) error {
	if i == nil || i.Index == nil {
		return nil
	}
	before := i.RangeCount()
	var msBefore runtime.MemStats
	runtime.ReadMemStats(&msBefore)

	ranges, err := LoadGeoRanges(ctx, i.ch)
	if err != nil {
		return err
	}
	clean, skipped := geoip.NormalizeRanges(ranges)
	if skipped > 0 {
		slog.Warn("geo index: overlapping or invalid ranges skipped", "skipped", skipped, "kept", len(clean))
	}
	i.ReplaceRanges(clean)

	var msAfter runtime.MemStats
	runtime.ReadMemStats(&msAfter)
	slog.Info("geo index loaded",
		"ranges", len(clean),
		"prev_ranges", before,
		"heap_alloc_mb", msAfter.Alloc/(1<<20),
		"heap_sys_mb", msAfter.Sys/(1<<20),
		"heap_delta_mb", int64(msAfter.Alloc-msBefore.Alloc)/(1<<20),
	)
	return nil
}
