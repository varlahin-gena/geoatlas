package anomalystore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"network_monitor/internal/adapter/clickhouse/query"
	"network_monitor/internal/adapter/clickhouse/sqlclause"
	usecaseanomaly "network_monitor/internal/usecase/anomaly"
)

func (r *Repository) OldestLogTime(ctx context.Context) (time.Time, error) {
	var t time.Time
	if r == nil || r.ch == nil {
		return t, fmt.Errorf("clickhouse not configured")
	}
	err := r.ch.QueryRow(ctx, `SELECT min(timestamp) FROM traffic_logs`).Scan(&t)
	return t, err
}

func (r *Repository) PortScan(ctx context.Context, window time.Duration, portsTh, eventsTh int, includePrivate bool, nets []usecaseanomaly.IPRange) ([]usecaseanomaly.PortScanHit, error) {
	touch, touchArgs := touchNetsSQL(nets)
	q := fmt.Sprintf(`
		SELECT toString(src_ip) AS src_ip, uniqExact(dst_port) AS ports, count() AS events, any(src_country) AS src_country
		FROM traffic_logs
		WHERE timestamp >= now() - INTERVAL %d SECOND
		  AND timestamp < now()
		  %s
		  %s
		GROUP BY src_ip
		HAVING ports >= ? AND events >= ?
		ORDER BY ports DESC
		LIMIT 10
		%s
	`, int(window.Seconds()), privateSrcSQL(includePrivate), touch, query.AggSettings())
	args := append(touchArgs, portsTh, eventsTh)
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []usecaseanomaly.PortScanHit
	for rows.Next() {
		var h usecaseanomaly.PortScanHit
		if err := rows.Scan(&h.SrcIP, &h.Ports, &h.Events, &h.SrcCountry); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *Repository) HorizontalScan(ctx context.Context, window time.Duration, hostsTh, eventsTh int, includePrivate bool, nets []usecaseanomaly.IPRange) ([]usecaseanomaly.HorizontalScanHit, error) {
	touch, touchArgs := touchNetsSQL(nets)
	q := fmt.Sprintf(`
		SELECT
			toString(src_ip) AS src_ip,
			concat(IPv4NumToString(bitAnd(toUInt32(dst_ip), toUInt32(4294967040))), '/24') AS net24,
			uniqExact(dst_ip) AS hosts,
			count() AS events
		FROM traffic_logs
		WHERE timestamp >= now() - INTERVAL %d SECOND
		  AND timestamp < now()
		  %s
		  %s
		GROUP BY src_ip, net24
		HAVING hosts >= ? AND events >= ?
		ORDER BY hosts DESC
		LIMIT 10
		%s
	`, int(window.Seconds()), privateSrcSQL(includePrivate), touch, query.AggSettings())
	args := append(touchArgs, hostsTh, eventsTh)
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []usecaseanomaly.HorizontalScanHit
	for rows.Next() {
		var h usecaseanomaly.HorizontalScanHit
		if err := rows.Scan(&h.SrcIP, &h.Net24, &h.Hosts, &h.Events); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *Repository) BlockedCount(ctx context.Context, start, end time.Time, net *usecaseanomaly.IPRange) (uint64, error) {
	touch, touchArgs := "", []any(nil)
	if net != nil {
		touch, touchArgs = touchNetsSQL([]usecaseanomaly.IPRange{*net})
	}
	q := fmt.Sprintf(`
		SELECT %s AS blocked
		FROM traffic_logs
		WHERE timestamp >= ? AND timestamp < ?
		  %s
	`, sqlclause.CountIfBlockedSQL(), touch)
	args := append([]any{start, end}, touchArgs...)
	var n uint64
	err := r.ch.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

func (r *Repository) CurrentCountries(ctx context.Context, window time.Duration, minN uint64, nets []usecaseanomaly.IPRange) ([]usecaseanomaly.CountryCount, error) {
	src, srcArgs := colInNetsSQL("src_ip", nets)
	q := fmt.Sprintf(`
		SELECT dst_country, count() AS n
		FROM traffic_logs
		WHERE timestamp >= now() - INTERVAL %d SECOND
		  AND dst_country != ''
		  AND dst_country NOT IN ('Неизвестно', 'Unknown', 'unknown', 'Reserved', 'reserved')
		  %s
		GROUP BY dst_country
		HAVING n >= ?
		ORDER BY n DESC
		LIMIT 50
	`, int(window.Seconds()), src)
	args := append(srcArgs, minN)
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []usecaseanomaly.CountryCount
	for rows.Next() {
		var c usecaseanomaly.CountryCount
		if err := rows.Scan(&c.Country, &c.N); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) CurrentCountryTotal(ctx context.Context, window time.Duration, nets []usecaseanomaly.IPRange) (uint64, error) {
	src, srcArgs := colInNetsSQL("src_ip", nets)
	q := fmt.Sprintf(`
		SELECT count()
		FROM traffic_logs
		WHERE timestamp >= now() - INTERVAL %d SECOND
		  AND dst_country != ''
		  AND dst_country NOT IN ('Неизвестно', 'Unknown', 'unknown', 'Reserved', 'reserved')
		  %s
	`, int(window.Seconds()), src)
	var total uint64
	err := r.ch.QueryRow(ctx, q, srcArgs...).Scan(&total)
	return total, err
}

func (r *Repository) BaselineCountries(ctx context.Context, days int, minN uint64, nets []usecaseanomaly.IPRange) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if len(nets) > 0 {
		src, srcArgs := colInNetsSQL("src_ip", nets)
		q := fmt.Sprintf(`
			SELECT dst_country
			FROM traffic_logs
			WHERE timestamp >= now() - INTERVAL %d DAY AND timestamp < toStartOfDay(now())
			  AND dst_country != ''
			  AND dst_country NOT IN ('Неизвестно', 'Unknown', 'unknown', 'Reserved', 'reserved')
			  %s
			GROUP BY dst_country
			HAVING count() >= ?
		`, days, src)
		args := append(srcArgs, minN)
		rows, err := r.ch.Query(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanCountrySet(rows, out)
	}
	q := `
		SELECT dst_key
		FROM traffic_edges_country_daily
		WHERE day >= today() - INTERVAL ? DAY AND day < today()
		  AND dst_key != '' AND dst_key != 'Неизвестно'
		GROUP BY dst_key
		HAVING sum(cnt) >= ?
	`
	rows, err := r.ch.Query(ctx, q, days, minN)
	if err != nil {
		q2 := fmt.Sprintf(`
			SELECT dst_country
			FROM traffic_logs
			WHERE timestamp >= now() - INTERVAL %d DAY AND timestamp < toStartOfDay(now())
			  AND dst_country != ''
			  AND dst_country NOT IN ('Неизвестно', 'Unknown', 'unknown', 'Reserved', 'reserved')
			GROUP BY dst_country
			HAVING count() >= ?
		`, days)
		rows, err = r.ch.Query(ctx, q2, minN)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()
	return scanCountrySet(rows, out)
}

func (r *Repository) RecentEdges(ctx context.Context, window time.Duration, limit int, nets []usecaseanomaly.IPRange) ([]usecaseanomaly.EdgeRow, error) {
	if limit < 1 {
		limit = 2000
	}
	src, srcArgs := colInNetsSQL("src_ip", nets)
	q := fmt.Sprintf(`
		SELECT toString(src_ip), toString(dst_ip), sum(cnt) AS cnt,
		       anyMerge(src_country), anyMerge(dst_country)
		FROM traffic_edges_hourly
		WHERE hour >= now() - INTERVAL %d SECOND
		  %s
		GROUP BY src_ip, dst_ip
		ORDER BY cnt DESC
		LIMIT %d
		%s
	`, int(window.Seconds()), src, limit, query.AggSettings())
	rows, err := r.ch.Query(ctx, q, srcArgs...)
	if err != nil {
		q2 := fmt.Sprintf(`
			SELECT toString(src_ip), toString(dst_ip), count() AS cnt,
			       any(src_country), any(dst_country)
			FROM traffic_logs
			WHERE timestamp >= now() - INTERVAL %d SECOND
			  %s
			GROUP BY src_ip, dst_ip
			ORDER BY cnt DESC
			LIMIT %d
			%s
		`, int(window.Seconds()), src, limit, query.AggSettings())
		rows, err = r.ch.Query(ctx, q2, srcArgs...)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()
	var out []usecaseanomaly.EdgeRow
	for rows.Next() {
		var e usecaseanomaly.EdgeRow
		if err := rows.Scan(&e.SrcIP, &e.DstIP, &e.Count, &e.SrcCountry, &e.DstCountry); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) KnownPairs(ctx context.Context, pairs [][2]string, lookback time.Duration) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if len(pairs) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(pairs))
	args := make([]any, 0, len(pairs)*2)
	for _, p := range pairs {
		if p[0] == "" || p[1] == "" {
			continue
		}
		placeholders = append(placeholders, "(?, ?)")
		args = append(args, p[0], p[1])
	}
	if len(placeholders) == 0 {
		return out, nil
	}
	q := fmt.Sprintf(`
		SELECT toString(src_ip), toString(dst_ip)
		FROM traffic_edges_hourly
		WHERE hour >= now() - INTERVAL %d SECOND
		  AND hour < toStartOfHour(now())
		  AND (src_ip, dst_ip) IN (%s)
		GROUP BY src_ip, dst_ip
	`, int(lookback.Seconds()), strings.Join(placeholders, ","))
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		q2 := fmt.Sprintf(`
			SELECT toString(src_ip), toString(dst_ip)
			FROM traffic_logs
			WHERE timestamp >= now() - INTERVAL %d SECOND
			  AND timestamp < toStartOfHour(now())
			  AND (src_ip, dst_ip) IN (%s)
			GROUP BY src_ip, dst_ip
		`, int(lookback.Seconds()), strings.Join(placeholders, ","))
		rows, err = r.ch.Query(ctx, q2, args...)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()
	for rows.Next() {
		var src, dst string
		if err := rows.Scan(&src, &dst); err != nil {
			return nil, err
		}
		out[src+"|"+dst] = struct{}{}
	}
	return out, rows.Err()
}

func scanCountrySet(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}, out map[string]struct{}) (map[string]struct{}, error) {
	if out == nil {
		out = map[string]struct{}{}
	}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		if c != "" {
			out[c] = struct{}{}
		}
	}
	return out, rows.Err()
}

func touchNetsSQL(nets []usecaseanomaly.IPRange) (string, []any) {
	return ipNetsSQL([]string{"src_ip", "dst_ip"}, nets)
}

func colInNetsSQL(col string, nets []usecaseanomaly.IPRange) (string, []any) {
	return ipNetsSQL([]string{col}, nets)
}

func ipNetsSQL(cols []string, nets []usecaseanomaly.IPRange) (string, []any) {
	if len(nets) == 0 || len(cols) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(nets)*len(cols))
	args := make([]any, 0, len(nets)*len(cols)*2)
	for _, n := range nets {
		if n.End < n.Start {
			continue
		}
		for _, col := range cols {
			parts = append(parts, fmt.Sprintf("(toUInt32(%s) BETWEEN ? AND ?)", col))
			args = append(args, n.Start, n.End)
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "AND (" + strings.Join(parts, " OR ") + ")", args
}

func privateSrcSQL(includePrivate bool) string {
	if includePrivate {
		return ""
	}
	return `
		AND NOT (
			(src_ip >= toIPv4('10.0.0.0') AND src_ip <= toIPv4('10.255.255.255'))
			OR (src_ip >= toIPv4('172.16.0.0') AND src_ip <= toIPv4('172.31.255.255'))
			OR (src_ip >= toIPv4('192.168.0.0') AND src_ip <= toIPv4('192.168.255.255'))
			OR (src_ip >= toIPv4('127.0.0.0') AND src_ip <= toIPv4('127.255.255.255'))
			OR (src_ip >= toIPv4('169.254.0.0') AND src_ip <= toIPv4('169.254.255.255'))
		)`
}

