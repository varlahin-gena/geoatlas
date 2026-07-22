package clickhouse

import (
	"context"
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/geoip"
	"network_monitor/internal/model"
)

// LoadGeoRanges читает geo_ranges из ClickHouse и нормализует индексный снимок.
func LoadGeoRanges(ctx context.Context, conn clickhouse.Conn) ([]model.GeoRange, error) {
	rows, err := conn.Query(ctx, `
		SELECT start_ip, end_ip, country, region, city, lat, lon
		FROM geo_ranges
		ORDER BY start_ip, end_ip
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ranges []model.GeoRange
	for rows.Next() {
		var g model.GeoRange
		if err := rows.Scan(&g.StartIP, &g.EndIP, &g.Country, &g.Region, &g.City, &g.Lat, &g.Lon); err != nil {
			return nil, err
		}
		ranges = append(ranges, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	clean, skipped := geoip.NormalizeRanges(ranges)
	if skipped > 0 {
		slog.Warn("geo index: overlapping or invalid ranges skipped", "skipped", skipped, "kept", len(clean))
	}
	return clean, nil
}

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
	clean, err := LoadGeoRanges(ctx, i.ch)
	if err != nil {
		return err
	}
	i.ReplaceRanges(clean)
	slog.Info("geo index loaded", "ranges", len(clean))
	return nil
}
