package clickhouse

import (
	"context"
	"log/slog"

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
	ranges, err := LoadGeoRanges(ctx, i.ch)
	if err != nil {
		return err
	}
	clean, skipped := geoip.NormalizeRanges(ranges)
	if skipped > 0 {
		slog.Warn("geo index: overlapping or invalid ranges skipped", "skipped", skipped, "kept", len(clean))
	}
	i.ReplaceRanges(clean)
	slog.Info("geo index loaded", "ranges", len(clean))
	return nil
}
