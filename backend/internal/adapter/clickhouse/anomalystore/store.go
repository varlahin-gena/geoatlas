package anomalystore

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/clickhouse/query"
	"network_monitor/internal/adapter/clickhouse/sqlclause"
	usecaseanomaly "network_monitor/internal/usecase/anomaly"
)

const zeroIPv4 = "0.0.0.0"

// Repository — ClickHouse store + traffic scans for anomaly engine.
type Repository struct {
	ch clickhouse.Conn
}

func New(ch clickhouse.Conn) *Repository {
	return &Repository{ch: ch}
}

var (
	_ usecaseanomaly.EventStore     = (*Repository)(nil)
	_ usecaseanomaly.TrafficScanner = (*Repository)(nil)
)

func (r *Repository) Insert(ctx context.Context, events []usecaseanomaly.Event) error {
	if r == nil || r.ch == nil || len(events) == 0 {
		return nil
	}
	batch, err := r.ch.PrepareBatch(ctx, `
		INSERT INTO anomaly_events (
			detected_at, window_start, window_end, code, severity, score, title, detail,
			src_ip, dst_ip, src_country, dst_country, src_city, dst_city, device,
			event_count, fingerprint, suppression_key, expires_at
		)
	`)
	if err != nil {
		return err
	}
	for _, e := range events {
		detail := "{}"
		if e.Detail != nil {
			b, err := json.Marshal(e.Detail)
			if err == nil {
				detail = string(b)
			}
		}
		src := e.SrcIP
		if src == "" {
			src = zeroIPv4
		}
		dst := e.DstIP
		if dst == "" {
			dst = zeroIPv4
		}
		if err := batch.Append(
			e.DetectedAt, e.WindowStart, e.WindowEnd, e.Code, e.Severity, e.Score, e.Title, detail,
			src, dst, e.SrcCountry, e.DstCountry, e.SrcCity, e.DstCity, e.Device,
			e.EventCount, e.Fingerprint, string(e.SuppressionKey), e.ExpiresAt,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func (r *Repository) List(ctx context.Context, q usecaseanomaly.ListQuery) ([]usecaseanomaly.Event, error) {
	if r == nil || r.ch == nil {
		return nil, fmt.Errorf("clickhouse not configured")
	}
	limit := q.Limit
	if limit < 1 || limit > 200 {
		return nil, fmt.Errorf("invalid anomalies limit: %d", limit)
	}
	var (
		sb   strings.Builder
		args []any
	)
	sb.WriteString(`
		SELECT
			e.detected_at, e.window_start, e.window_end, e.code, e.severity, e.score,
			e.title, e.detail, toString(e.src_ip), toString(e.dst_ip),
			e.src_country, e.dst_country, e.src_city, e.dst_city, e.device,
			e.event_count, e.fingerprint, e.expires_at,
			if(a.fingerprint = '', 0, 1) AS acked
		FROM anomaly_events AS e
		LEFT JOIN (
			SELECT fingerprint
			FROM anomaly_acks
			GROUP BY fingerprint
		) AS a ON e.fingerprint = a.fingerprint
		WHERE e.detected_at >= ?
	`)
	args = append(args, q.Since)
	if sev := strings.TrimSpace(q.Severity); sev != "" {
		sb.WriteString(" AND e.severity = ?")
		args = append(args, sev)
	}
	if code := strings.TrimSpace(q.Code); code != "" {
		sb.WriteString(" AND e.code = ?")
		args = append(args, code)
	}
	if !q.IncludeAcked {
		sb.WriteString(" AND a.fingerprint = ''")
	}
	sb.WriteString(" ORDER BY e.score DESC, e.detected_at DESC LIMIT ?")
	args = append(args, uint64(limit))

	rows, err := r.ch.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]usecaseanomaly.Event, 0, limit)
	for rows.Next() {
		var (
			e      usecaseanomaly.Event
			detail string
			acked  uint8
		)
		if err := rows.Scan(
			&e.DetectedAt, &e.WindowStart, &e.WindowEnd, &e.Code, &e.Severity, &e.Score,
			&e.Title, &detail, &e.SrcIP, &e.DstIP,
			&e.SrcCountry, &e.DstCountry, &e.SrcCity, &e.DstCity, &e.Device,
			&e.EventCount, &e.Fingerprint, &e.ExpiresAt, &acked,
		); err != nil {
			return nil, err
		}
		e.Acknowledged = acked != 0
		e.SrcIP = displayIP(e.SrcIP)
		e.DstIP = displayIP(e.DstIP)
		if detail != "" && detail != "{}" {
			_ = json.Unmarshal([]byte(detail), &e.Detail)
		}
		e.Map = mapLinkFromEvent(e)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) ExistingFingerprints(ctx context.Context, fps []string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if r == nil || r.ch == nil || len(fps) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(fps))
	args := make([]any, len(fps))
	for i, fp := range fps {
		placeholders[i] = "?"
		args[i] = fp
	}
	q := `SELECT DISTINCT fingerprint FROM anomaly_events WHERE fingerprint IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var fp string
		if err := rows.Scan(&fp); err != nil {
			return nil, err
		}
		out[fp] = struct{}{}
	}
	return out, rows.Err()
}

func (r *Repository) ActiveSuppressions(ctx context.Context, keys []usecaseanomaly.SuppressionKey, now time.Time) (map[usecaseanomaly.SuppressionKey]struct{}, error) {
	out := map[usecaseanomaly.SuppressionKey]struct{}{}
	if r == nil || r.ch == nil || len(keys) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys)+1)
	for _, key := range keys {
		if key == "" {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, string(key))
	}
	if len(placeholders) == 0 {
		return out, nil
	}
	args = append(args, now.UTC())
	q := `SELECT suppression_key FROM anomaly_suppressions WHERE suppression_key IN (` + strings.Join(placeholders, ",") + `) AND suppressed_until > ? GROUP BY suppression_key`
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out[usecaseanomaly.SuppressionKey(key)] = struct{}{}
	}
	return out, rows.Err()
}

func (r *Repository) RecentSuppressionKeys(ctx context.Context, code string, keys []usecaseanomaly.SuppressionKey, since time.Time) (map[usecaseanomaly.SuppressionKey]struct{}, error) {
	out := map[usecaseanomaly.SuppressionKey]struct{}{}
	if r == nil || r.ch == nil || len(keys) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys)+2)
	args = append(args, code, since.UTC())
	for _, key := range keys {
		if key == "" {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, string(key))
	}
	if len(placeholders) == 0 {
		return out, nil
	}
	q := `SELECT suppression_key FROM anomaly_events WHERE code = ? AND detected_at >= ? AND suppression_key IN (` + strings.Join(placeholders, ",") + `) GROUP BY suppression_key`
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out[usecaseanomaly.SuppressionKey(key)] = struct{}{}
	}
	return out, rows.Err()
}

func (r *Repository) Ack(ctx context.Context, fingerprint, by string, suppressFor time.Duration) error {
	if r == nil || r.ch == nil {
		return fmt.Errorf("clickhouse not configured")
	}
	now := time.Now().UTC()
	if err := r.ch.Exec(ctx, `
		INSERT INTO anomaly_acks (fingerprint, ack_at, ack_by) VALUES (?, ?, ?)
	`, fingerprint, now, by); err != nil {
		return err
	}
	if suppressFor <= 0 {
		return nil
	}
	var (
		code string
		key  string
	)
	err := r.ch.QueryRow(ctx, `
		SELECT code, suppression_key
		FROM anomaly_events
		WHERE fingerprint = ?
		ORDER BY detected_at DESC
		LIMIT 1
	`, fingerprint).Scan(&code, &key)
	if err != nil || strings.TrimSpace(key) == "" {
		return err
	}
	return r.ch.Exec(ctx, `
		INSERT INTO anomaly_suppressions
		(suppression_key, code, source_fingerprint, suppressed_at, suppressed_until, suppressed_by)
		VALUES (?, ?, ?, ?, ?, ?)
	`, key, code, fingerprint, now, now.Add(suppressFor), by)
}

func (r *Repository) CountSummary(ctx context.Context, since time.Time) (usecaseanomaly.Summary, error) {
	var sum usecaseanomaly.Summary
	if r == nil || r.ch == nil {
		return sum, fmt.Errorf("clickhouse not configured")
	}
	var total, high, warn, acked uint64
	err := r.ch.QueryRow(ctx, `
		SELECT
			countIf(a.fingerprint = '') AS total,
			countIf(e.severity = 'high' AND a.fingerprint = '') AS high,
			countIf(e.severity = 'warn' AND a.fingerprint = '') AS warn,
			countIf(a.fingerprint != '') AS acked
		FROM anomaly_events AS e
		LEFT JOIN (
			SELECT fingerprint FROM anomaly_acks GROUP BY fingerprint
		) AS a ON e.fingerprint = a.fingerprint
		WHERE e.detected_at >= ?
	`, since).Scan(&total, &high, &warn, &acked)
	if err != nil {
		return sum, err
	}
	sum.Total = int(total)
	sum.High = int(high)
	sum.Warn = int(warn)
	sum.Acked = int(acked)
	return sum, nil
}

func (r *Repository) OldestLogTime(ctx context.Context) (time.Time, error) {
	var t time.Time
	if r == nil || r.ch == nil {
		return t, fmt.Errorf("clickhouse not configured")
	}
	err := r.ch.QueryRow(ctx, `SELECT min(timestamp) FROM traffic_logs`).Scan(&t)
	return t, err
}

func (r *Repository) PortScan(ctx context.Context, window time.Duration, portsTh, eventsTh int, includePrivate bool) ([]usecaseanomaly.PortScanHit, error) {
	q := fmt.Sprintf(`
		SELECT toString(src_ip) AS src_ip, uniqExact(dst_port) AS ports, count() AS events, any(src_country) AS src_country
		FROM traffic_logs
		WHERE timestamp >= now() - INTERVAL %d SECOND
		  AND timestamp < now()
		  %s
		GROUP BY src_ip
		HAVING ports >= ? AND events >= ?
		ORDER BY ports DESC
		LIMIT 10
		%s
	`, int(window.Seconds()), privateSrcSQL(includePrivate), query.AggSettings())
	rows, err := r.ch.Query(ctx, q, portsTh, eventsTh)
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

func (r *Repository) HorizontalScan(ctx context.Context, window time.Duration, hostsTh, eventsTh int, includePrivate bool) ([]usecaseanomaly.HorizontalScanHit, error) {
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
		GROUP BY src_ip, net24
		HAVING hosts >= ? AND events >= ?
		ORDER BY hosts DESC
		LIMIT 10
		%s
	`, int(window.Seconds()), privateSrcSQL(includePrivate), query.AggSettings())
	rows, err := r.ch.Query(ctx, q, hostsTh, eventsTh)
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

func (r *Repository) BlockedCount(ctx context.Context, start, end time.Time) (uint64, error) {
	q := fmt.Sprintf(`
		SELECT %s AS blocked
		FROM traffic_logs
		WHERE timestamp >= ? AND timestamp < ?
	`, sqlclause.CountIfBlockedSQL())
	var n uint64
	err := r.ch.QueryRow(ctx, q, start, end).Scan(&n)
	return n, err
}

func (r *Repository) CurrentCountries(ctx context.Context, window time.Duration, minN uint64) ([]usecaseanomaly.CountryCount, error) {
	q := fmt.Sprintf(`
		SELECT dst_country, count() AS n
		FROM traffic_logs
		WHERE timestamp >= now() - INTERVAL %d SECOND
		  AND dst_country != ''
		  AND dst_country NOT IN ('Неизвестно', 'Unknown', 'unknown', 'Reserved', 'reserved')
		GROUP BY dst_country
		HAVING n >= ?
		ORDER BY n DESC
		LIMIT 50
	`, int(window.Seconds()))
	rows, err := r.ch.Query(ctx, q, minN)
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

func (r *Repository) CurrentCountryTotal(ctx context.Context, window time.Duration) (uint64, error) {
	q := fmt.Sprintf(`
		SELECT count()
		FROM traffic_logs
		WHERE timestamp >= now() - INTERVAL %d SECOND
		  AND dst_country != ''
		  AND dst_country NOT IN ('Неизвестно', 'Unknown', 'unknown', 'Reserved', 'reserved')
	`, int(window.Seconds()))
	var total uint64
	err := r.ch.QueryRow(ctx, q).Scan(&total)
	return total, err
}

func (r *Repository) BaselineCountries(ctx context.Context, days int, minN uint64) (map[string]struct{}, error) {
	out := map[string]struct{}{}
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
		// таблица ещё не создана — фолбэк на raw
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

func (r *Repository) RecentEdges(ctx context.Context, window time.Duration, limit int) ([]usecaseanomaly.EdgeRow, error) {
	if limit < 1 {
		limit = 2000
	}
	q := fmt.Sprintf(`
		SELECT toString(src_ip), toString(dst_ip), sum(cnt) AS cnt,
		       anyMerge(src_country), anyMerge(dst_country)
		FROM traffic_edges_hourly
		WHERE hour >= now() - INTERVAL %d SECOND
		GROUP BY src_ip, dst_ip
		ORDER BY cnt DESC
		LIMIT %d
		%s
	`, int(window.Seconds()), limit, query.AggSettings())
	rows, err := r.ch.Query(ctx, q)
	if err != nil {
		q2 := fmt.Sprintf(`
			SELECT toString(src_ip), toString(dst_ip), count() AS cnt,
			       any(src_country), any(dst_country)
			FROM traffic_logs
			WHERE timestamp >= now() - INTERVAL %d SECOND
			GROUP BY src_ip, dst_ip
			ORDER BY cnt DESC
			LIMIT %d
			%s
		`, int(window.Seconds()), limit, query.AggSettings())
		rows, err = r.ch.Query(ctx, q2)
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

func displayIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" || ip == zeroIPv4 {
		return ""
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	return ip
}

func mapLinkFromEvent(e usecaseanomaly.Event) usecaseanomaly.MapLink {
	switch e.Code {
	case usecaseanomaly.CodePortScan, usecaseanomaly.CodeHorizontalScan:
		if e.SrcIP != "" {
			return usecaseanomaly.MapLink{Period: "15m", Group: "ip", Filter: "all", Query: "src:" + e.SrcIP}
		}
	case usecaseanomaly.CodeRepNewDst:
		q := ""
		if e.SrcIP != "" && e.DstIP != "" {
			q = "src:" + e.SrcIP + " dst:" + e.DstIP
		}
		return usecaseanomaly.MapLink{Period: "1h", Group: "ip", Filter: "all", Query: q}
	case usecaseanomaly.CodeNewCountryDst:
		return usecaseanomaly.MapLink{Period: "1h", Group: "country", Filter: "all", Query: "dst:" + e.DstCountry, Country: e.DstCountry}
	case usecaseanomaly.CodeBlockedSurge:
		return usecaseanomaly.MapLink{Period: "1h", Group: "country", Filter: "blocked"}
	}
	return usecaseanomaly.MapLink{Period: "1h", Group: "city", Filter: "all"}
}
