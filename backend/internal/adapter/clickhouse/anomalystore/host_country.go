package anomalystore

import (
	"context"
	"fmt"
	"time"

	"geoatlas/internal/adapter/clickhouse/query"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
)

const countryExcludeSQL = `AND dst_country != ''
		  AND dst_country NOT IN ('Неизвестно', 'Unknown', 'unknown', 'Reserved', 'reserved')`

func (r *Repository) HostCountries(ctx context.Context, window time.Duration, minN uint64, nets []usecaseanomaly.IPRange) ([]usecaseanomaly.HostCountryHit, error) {
	src, srcArgs := colInNetsSQL("src_ip", nets)
	q := fmt.Sprintf(`
		SELECT toString(src_ip), dst_country, count() AS n
		FROM traffic_logs
		WHERE timestamp >= now() - INTERVAL %d SECOND
		  %s
		  %s
		GROUP BY src_ip, dst_country
		HAVING n >= ?
		ORDER BY n DESC
		LIMIT 50
		%s
	`, int(window.Seconds()), countryExcludeSQL, src, query.AggSettings())
	args := append(srcArgs, minN)
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []usecaseanomaly.HostCountryHit
	for rows.Next() {
		var h usecaseanomaly.HostCountryHit
		if err := rows.Scan(&h.SrcIP, &h.Country, &h.Events); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *Repository) HostCountryBaseline(ctx context.Context, days int, minN uint64, nets []usecaseanomaly.IPRange) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	src, srcArgs := colInNetsSQL("src_ip", nets)
	q := fmt.Sprintf(`
		SELECT toString(src_ip), dst_country
		FROM traffic_logs
		WHERE timestamp >= now() - INTERVAL %d DAY
		  AND timestamp < now() - INTERVAL 1 HOUR
		  %s
		  %s
		GROUP BY src_ip, dst_country
		HAVING count() >= ?
	`, days, countryExcludeSQL, src)
	args := append(srcArgs, minN)
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var srcIP, country string
		if err := rows.Scan(&srcIP, &country); err != nil {
			return nil, err
		}
		if srcIP == "" || country == "" {
			continue
		}
		out[srcIP+"|"+country] = struct{}{}
	}
	return out, rows.Err()
}
