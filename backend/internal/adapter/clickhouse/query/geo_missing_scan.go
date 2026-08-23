package query

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"geoatlas/internal/model"
)

// ScanGeoMissingIPsForTimeRange — лёгкий отчёт: IP с нулевыми lat/lon в логах.
// Тяжёлый ScanRawAggs здесь не нужен: страница geo-missing только ищет «дыры» в GeoIP.
func ScanGeoMissingIPsForTimeRange(
	ctx context.Context,
	ch clickhouse.Conn,
	tr model.TimeRange,
	limit int,
	timeout time.Duration,
) ([]model.GeoMissingIPRow, error) {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}

	var (
		where string
		args  []any
	)
	switch tr.Mode {
	case "minutes":
		where = "timestamp >= now() - INTERVAL ? MINUTE"
		args = []any{tr.Amount}
	case "hours":
		where = "timestamp >= now() - INTERVAL ? HOUR"
		args = []any{tr.Amount}
	case "days":
		where = "timestamp >= now() - INTERVAL ? DAY"
		args = []any{tr.Amount}
	case "absolute":
		if !tr.To.After(tr.From) {
			return nil, nil
		}
		where = "timestamp >= ? AND timestamp < ?"
		args = []any{tr.From, tr.To}
	default:
		where = "timestamp >= now() - INTERVAL ? DAY"
		args = []any{1}
	}

	q := fmt.Sprintf(`
		SELECT
			ip,
			sum(cnt) AS cnt,
			sum(as_src) AS as_src,
			sum(as_dst) AS as_dst,
			any(peer) AS peer,
			anyIf(country, country != '' AND country != 'Неизвестно') AS country,
			anyIf(city, city != '') AS city,
			-- Подзапрос уже HAVING lat=0 AND lon=0; anyIf здесь только рискует alias nesting.
			any(lat) AS out_lat,
			any(lon) AS out_lon
		FROM (
			SELECT
				toString(src_ip) AS ip,
				count() AS cnt,
				count() AS as_src,
				toUInt64(0) AS as_dst,
				any(toString(dst_ip)) AS peer,
				any(src_country) AS country,
				anyIf(src_city, src_city != '') AS city,
				anyIf(src_lat, (src_lat != 0) OR (src_lon != 0)) AS lat,
				anyIf(src_lon, (src_lat != 0) OR (src_lon != 0)) AS lon
			FROM traffic_logs
			WHERE %s
			GROUP BY src_ip
			HAVING lat = 0 AND lon = 0

			UNION ALL

			SELECT
				toString(dst_ip) AS ip,
				count() AS cnt,
				toUInt64(0) AS as_src,
				count() AS as_dst,
				any(toString(src_ip)) AS peer,
				any(dst_country) AS country,
				anyIf(dst_city, dst_city != '') AS city,
				anyIf(dst_lat, (dst_lat != 0) OR (dst_lon != 0)) AS lat,
				anyIf(dst_lon, (dst_lat != 0) OR (dst_lon != 0)) AS lon
			FROM traffic_logs
			WHERE %s
			GROUP BY dst_ip
			HAVING lat = 0 AND lon = 0
		)
		GROUP BY ip
		ORDER BY cnt DESC
		%s
		%s
	`, where, where, limitClause(limit), AggSettings())

	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// args duplicated for two WHERE clauses
	queryArgs := append(append([]any{}, args...), args...)
	rows, err := ch.Query(qctx, q, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.GeoMissingIPRow
	for rows.Next() {
		var r model.GeoMissingIPRow
		if err := rows.Scan(
			&r.IP, &r.Count, &r.AsSrc, &r.AsDst, &r.SamplePeer,
			&r.LogCountry, &r.LogCity, &r.LogLat, &r.LogLon,
		); err != nil {
			slog.Warn("ScanGeoMissingIPs: row scan failed, skipping", "err", err)
			continue
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
