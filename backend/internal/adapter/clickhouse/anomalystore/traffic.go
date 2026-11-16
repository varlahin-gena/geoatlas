package anomalystore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"geoatlas/internal/adapter/clickhouse/query"
	"geoatlas/internal/adapter/clickhouse/sqlclause"
	"geoatlas/internal/model"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
)

func (r *Repository) OldestLogTime(ctx context.Context) (time.Time, error) {
	var t time.Time
	if r == nil || r.ch == nil {
		return t, fmt.Errorf("clickhouse not configured")
	}
	err := r.ch.QueryRow(ctx, `SELECT min(timestamp) FROM traffic_logs`).Scan(&t)
	return t, err
}

func (r *Repository) PortScan(ctx context.Context, window time.Duration, portsTh, eventsTh int, nets []usecaseanomaly.IPRange) ([]usecaseanomaly.PortScanHit, error) {
	touch, touchArgs := touchNetsSQL(nets)
	blocked := fmt.Sprintf("AND lower(action) IN (%s)", model.BlockedInClause())
	skip, skipArgs := notMatch(ctx, usecaseanomaly.CodePortScan)
	q := fmt.Sprintf(`
		SELECT toString(src_ip) AS src_ip, uniqExact(dst_port) AS ports, count() AS events, any(src_country) AS src_country
		FROM traffic_logs
		WHERE timestamp >= now() - INTERVAL %d SECOND
		  AND timestamp < now()
		  %s
		  %s
		  %s
		GROUP BY src_ip
		HAVING ports >= ? AND events >= ?
		ORDER BY ports DESC
		LIMIT 10
		%s
	`, int(window.Seconds()), blocked, touch, skip, query.AggSettings())
	args := append(append(touchArgs, skipArgs...), portsTh, eventsTh)
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

func (r *Repository) BlockedCount(ctx context.Context, start, end time.Time, net *usecaseanomaly.IPRange) (uint64, error) {
	touch, touchArgs := "", []any(nil)
	if net != nil {
		touch, touchArgs = touchNetsSQL([]usecaseanomaly.IPRange{*net})
	}
	skip, skipArgs := notMatch(ctx, usecaseanomaly.CodeBlockedSurge)
	q := fmt.Sprintf(`
		SELECT %s AS blocked
		FROM traffic_logs
		WHERE timestamp >= ? AND timestamp < ?
		  %s
		  %s
	`, sqlclause.CountIfBlockedSQL(), touch, skip)
	args := append(append([]any{start, end}, touchArgs...), skipArgs...)
	var n uint64
	err := r.ch.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

// BlockedCounts считает current и previous окна для каждой сети одним SELECT
// (чанки по 32, чтобы не раздувать список countIf). Порядок совпадает с nets.
func (r *Repository) BlockedCounts(ctx context.Context, currStart, currEnd, prevStart, prevEnd time.Time, nets []usecaseanomaly.IPRange) ([]usecaseanomaly.BlockedWindowCount, error) {
	if len(nets) == 0 {
		return nil, nil
	}
	out := make([]usecaseanomaly.BlockedWindowCount, len(nets))
	const chunk = 32
	for i := 0; i < len(nets); i += chunk {
		j := i + chunk
		if j > len(nets) {
			j = len(nets)
		}
		if err := r.scanBlockedChunk(ctx, currStart, currEnd, prevStart, prevEnd, nets[i:j], out[i:j]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Repository) scanBlockedChunk(ctx context.Context, currStart, currEnd, prevStart, prevEnd time.Time, nets []usecaseanomaly.IPRange, dest []usecaseanomaly.BlockedWindowCount) error {
	action := fmt.Sprintf("lower(action) IN (%s)", model.BlockedInClause())
	src := ipv4AsUInt32SQL("src_ip")
	dst := ipv4AsUInt32SQL("dst_ip")
	pred := fmt.Sprintf(
		"countIf(timestamp >= ? AND timestamp < ? AND %s AND (%s BETWEEN ? AND ? OR %s BETWEEN ? AND ?))",
		action, src, dst,
	)
	cols := make([]string, 0, len(nets)*2)
	args := make([]any, 0, len(nets)*12+2)
	for _, n := range nets {
		cols = append(cols, pred, pred)
		args = append(args,
			currStart, currEnd, n.Start, n.End, n.Start, n.End,
			prevStart, prevEnd, n.Start, n.End, n.Start, n.End,
		)
	}
	args = append(args, prevStart, currEnd)
	skip, skipArgs := notMatch(ctx, usecaseanomaly.CodeBlockedSurge)
	args = append(args, skipArgs...)
	q := fmt.Sprintf(`SELECT %s FROM traffic_logs WHERE timestamp >= ? AND timestamp < ? %s`, strings.Join(cols, ", "), skip)
	vals := make([]uint64, len(nets)*2)
	ptrs := make([]any, len(vals))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := r.ch.QueryRow(ctx, q, args...).Scan(ptrs...); err != nil {
		return err
	}
	for i := range nets {
		dest[i] = usecaseanomaly.BlockedWindowCount{Current: vals[i*2], Previous: vals[i*2+1]}
	}
	return nil
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

func (r *Repository) ReputationEdges(ctx context.Context, window time.Duration, minEvents uint64, limit int, nets []usecaseanomaly.IPRange) ([]usecaseanomaly.EdgeRow, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	sec := int(window.Seconds())
	src, srcArgs := colInNetsSQL("src_ip", nets)
	srcU := ipv4AsUInt32SQL("e.src_ip")
	dstU := ipv4AsUInt32SQL("e.dst_ip")
	skip, skipArgs := notMatch(ctx, usecaseanomaly.CodeRepNewDst)
	hourlyArgs := append(append([]any{}, srcArgs...), minEvents)
	q := fmt.Sprintf(`
		SELECT toString(e.src_ip), toString(e.dst_ip), any(e.cnt) AS cnt,
		       any(e.src_country), any(e.dst_country)
		FROM (
			SELECT src_ip, dst_ip, sum(cnt) AS cnt,
			       anyMerge(src_country) AS src_country,
			       anyMerge(dst_country) AS dst_country
			FROM traffic_edges_hourly
			WHERE hour >= now() - INTERVAL %d SECOND
			  %s
			GROUP BY src_ip, dst_ip
			HAVING cnt >= ?
		) AS e
		INNER JOIN reputation_ranges AS r
		  ON (%s BETWEEN r.start_ip AND r.end_ip OR %s BETWEEN r.start_ip AND r.end_ip)
		GROUP BY e.src_ip, e.dst_ip
		ORDER BY cnt DESC
		LIMIT %d
		%s
	`, sec, src, srcU, dstU, limit, query.AggSettings())
	var rows driver.Rows
	var err error
	if skip == "" {
		rows, err = r.ch.Query(ctx, q, hourlyArgs...)
	}
	if skip != "" || err != nil {
		if skip == "" && !shouldFallbackToLogs(ctx, err) {
			return nil, err
		}
		q2 := fmt.Sprintf(`
			SELECT toString(e.src_ip), toString(e.dst_ip), any(e.cnt) AS cnt,
			       any(e.src_country), any(e.dst_country)
			FROM (
				SELECT src_ip, dst_ip, count() AS cnt,
				       any(src_country) AS src_country,
				       any(dst_country) AS dst_country
				FROM traffic_logs
				WHERE timestamp >= now() - INTERVAL %d SECOND
				  %s
				  %s
				GROUP BY src_ip, dst_ip
				HAVING cnt >= ?
			) AS e
			INNER JOIN reputation_ranges AS r
			  ON (%s BETWEEN r.start_ip AND r.end_ip OR %s BETWEEN r.start_ip AND r.end_ip)
			GROUP BY e.src_ip, e.dst_ip
			ORDER BY cnt DESC
			LIMIT %d
			%s
		`, sec, src, skip, srcU, dstU, limit, query.AggSettings())
		logArgs := append(append(append([]any{}, srcArgs...), skipArgs...), minEvents)
		rows, err = r.ch.Query(ctx, q2, logArgs...)
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

func (r *Repository) ByteSurge(ctx context.Context, window time.Duration, floor, absMin uint64, nets []usecaseanomaly.IPRange) ([]usecaseanomaly.ByteSurgeHit, error) {
	if window < time.Minute {
		window = time.Hour
	}
	sec := int(window.Seconds())
	skip, skipArgs := notMatch(ctx, usecaseanomaly.CodeByteSurge)
	var rows driver.Rows
	var err error
	if skip == "" {
		rows, err = r.ch.Query(ctx, byteSurgeSQL(true, sec, nets, ""), byteSurgeArgs(absMin, floor, nets, nil)...)
	}
	if skip != "" || err != nil {
		if skip == "" && !shouldFallbackToLogs(ctx, err) {
			return nil, err
		}
		rows, err = r.ch.Query(ctx, byteSurgeSQL(false, sec, nets, skip), byteSurgeArgs(absMin, floor, nets, skipArgs)...)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()
	var out []usecaseanomaly.ByteSurgeHit
	for rows.Next() {
		var h usecaseanomaly.ByteSurgeHit
		if err := rows.Scan(&h.HostIP, &h.Peer, &h.BytesNow, &h.BytesPrev, &h.Events, &h.Direction); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *Repository) Beaconing(ctx context.Context, lookback time.Duration, minHours int, maxAvgBytes uint64, nets []usecaseanomaly.IPRange) ([]usecaseanomaly.BeaconingHit, error) {
	if lookback < time.Hour {
		lookback = 24 * time.Hour
	}
	if minHours < 2 {
		minHours = 2
	}
	sec := int(lookback.Seconds())
	touch, touchArgs := touchNetsSQL(nets)
	skip, skipArgs := notMatch(ctx, usecaseanomaly.CodeBeaconing)
	args := append(append([]any{}, touchArgs...), minHours, maxAvgBytes)

	// Phase 1: candidate pairs without hour arrays (groupUniqArray over all pairs is too heavy).
	// Исключение считается по журналу: в часовом агрегате страна и действие не обычные колонки.
	if skip != "" {
		logArgs := append(append(append([]any{}, touchArgs...), skipArgs...), minHours, maxAvgBytes)
		qLogs := fmt.Sprintf(`
			SELECT
				toString(src_ip) AS src_ip,
				toString(dst_ip) AS dst_ip,
				uniqExact(toStartOfHour(timestamp)) AS active_hours,
				sum(bytes_sent) + sum(bytes_recv) AS total_bytes,
				count() AS events
			FROM traffic_logs
			WHERE timestamp >= now() - INTERVAL %d SECOND
			  %s
			  %s
			GROUP BY src_ip, dst_ip
			HAVING active_hours >= ?
			  AND (total_bytes / active_hours) <= ?
			ORDER BY active_hours DESC, total_bytes ASC
			LIMIT 30
			%s
		`, sec, touch, skip, query.AggSettings())
		cands, err := r.scanBeaconingCandidates(ctx, qLogs, logArgs)
		if err != nil {
			return nil, err
		}
		return r.attachBeaconingHours(ctx, cands, sec, true)
	}
	qHourly := fmt.Sprintf(`
		SELECT
			toString(src_ip) AS src_ip,
			toString(dst_ip) AS dst_ip,
			uniqExact(hour) AS active_hours,
			sum(bytes_sent) + sum(bytes_recv) AS total_bytes,
			sum(cnt) AS events
		FROM traffic_edges_hourly
		WHERE hour >= now() - INTERVAL %d SECOND
		  %s
		GROUP BY src_ip, dst_ip
		HAVING active_hours >= ?
		  AND (total_bytes / active_hours) <= ?
		ORDER BY active_hours DESC, total_bytes ASC
		LIMIT 30
		%s
	`, sec, touch, query.AggSettings())

	cands, err := r.scanBeaconingCandidates(ctx, qHourly, args)
	if err != nil {
		if !shouldFallbackToLogs(ctx, err) {
			return nil, err
		}
		qLogs := fmt.Sprintf(`
			SELECT
				toString(src_ip) AS src_ip,
				toString(dst_ip) AS dst_ip,
				uniqExact(toStartOfHour(timestamp)) AS active_hours,
				sum(bytes_sent) + sum(bytes_recv) AS total_bytes,
				count() AS events
			FROM traffic_logs
			WHERE timestamp >= now() - INTERVAL %d SECOND
			  %s
			GROUP BY src_ip, dst_ip
			HAVING active_hours >= ?
			  AND (total_bytes / active_hours) <= ?
			ORDER BY active_hours DESC, total_bytes ASC
			LIMIT 30
			%s
		`, sec, touch, query.AggSettings())
		cands, err = r.scanBeaconingCandidates(ctx, qLogs, args)
		if err != nil {
			return nil, err
		}
		return r.attachBeaconingHours(ctx, cands, sec, true)
	}
	return r.attachBeaconingHours(ctx, cands, sec, false)
}

type beaconingCand struct {
	SrcIP       string
	DstIP       string
	ActiveHours uint64
	TotalBytes  uint64
	Events      uint64
}

func (r *Repository) scanBeaconingCandidates(ctx context.Context, q string, args []any) ([]beaconingCand, error) {
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []beaconingCand
	for rows.Next() {
		var c beaconingCand
		if err := rows.Scan(&c.SrcIP, &c.DstIP, &c.ActiveHours, &c.TotalBytes, &c.Events); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) attachBeaconingHours(ctx context.Context, cands []beaconingCand, lookbackSec int, fromLogs bool) ([]usecaseanomaly.BeaconingHit, error) {
	out := make([]usecaseanomaly.BeaconingHit, 0, len(cands))
	if len(cands) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(cands))
	args := make([]any, 0, len(cands)*2)
	index := make(map[string]int, len(cands))
	for i, c := range cands {
		placeholders = append(placeholders, "(?, ?)")
		args = append(args, c.SrcIP, c.DstIP)
		index[c.SrcIP+"\x00"+c.DstIP] = i
		out = append(out, usecaseanomaly.BeaconingHit{
			SrcIP: c.SrcIP, DstIP: c.DstIP,
			ActiveHours: c.ActiveHours, TotalBytes: c.TotalBytes, Events: c.Events,
		})
	}
	var q string
	if fromLogs {
		q = fmt.Sprintf(`
			SELECT toString(src_ip), toString(dst_ip),
				arraySort(groupUniqArray(toUnixTimestamp(toStartOfHour(timestamp))))
			FROM traffic_logs
			WHERE timestamp >= now() - INTERVAL %d SECOND
			  AND (toString(src_ip), toString(dst_ip)) IN (%s)
			GROUP BY src_ip, dst_ip
			%s
		`, lookbackSec, strings.Join(placeholders, ","), query.AggSettings())
	} else {
		q = fmt.Sprintf(`
			SELECT toString(src_ip), toString(dst_ip),
				arraySort(groupUniqArray(toUnixTimestamp(toStartOfHour(hour))))
			FROM traffic_edges_hourly
			WHERE hour >= now() - INTERVAL %d SECOND
			  AND (toString(src_ip), toString(dst_ip)) IN (%s)
			GROUP BY src_ip, dst_ip
			%s
		`, lookbackSec, strings.Join(placeholders, ","), query.AggSettings())
	}
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		// Candidates alone are enough for emit with regularity=0 filter skipping;
		// return without hours rather than failing the whole detector.
		return out, nil
	}
	defer rows.Close()
	for rows.Next() {
		var src, dst string
		var hours []int64
		if err := rows.Scan(&src, &dst, &hours); err != nil {
			return nil, err
		}
		if i, ok := index[src+"\x00"+dst]; ok {
			out[i].HourUnix = hours
		}
	}
	return out, rows.Err()
}

// shouldFallbackToLogs — только отсутствие hourly-таблицы; не при timeout/type errors.
func shouldFallbackToLogs(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var ex *clickhouse.Exception
	if errors.As(err, &ex) {
		return ex.Code == 60 // UNKNOWN_TABLE
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unknown table") || strings.Contains(msg, "doesn't exist")
}

func (r *Repository) LateralFanout(ctx context.Context, window time.Duration, hostsTh, eventsTh int, nets []usecaseanomaly.IPRange, exclude []string) ([]usecaseanomaly.LateralFanoutHit, error) {
	dst, dstArgs := colInNetsSQL("dst_ip", nets)
	ex, exArgs := excludeSrcSQL(exclude)
	skip, skipArgs := notMatch(ctx, usecaseanomaly.CodeLateralFanout)
	q := fmt.Sprintf(`
		SELECT toString(src_ip) AS src_ip, uniqExact(dst_ip) AS hosts, count() AS events
		FROM traffic_logs
		WHERE timestamp >= now() - INTERVAL %d SECOND
		  AND timestamp < now()
		  %s
		  %s
		  %s
		GROUP BY src_ip
		HAVING hosts >= ? AND events >= ?
		ORDER BY hosts DESC
		LIMIT 10
		%s
	`, int(window.Seconds()), dst, ex, skip, query.AggSettings())
	args := append(append(append(dstArgs, exArgs...), skipArgs...), hostsTh, eventsTh)
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []usecaseanomaly.LateralFanoutHit
	for rows.Next() {
		var h usecaseanomaly.LateralFanoutHit
		if err := rows.Scan(&h.SrcIP, &h.Hosts, &h.Events); err != nil {
			return nil, err
		}
		out = append(out, h)
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

// ipv4AsUInt32SQL converts IPv4 or String IP columns to UInt32.
// Plain toUInt32(col) fails on String IPs like '10.1.0.1' (parses only '10').
func ipv4AsUInt32SQL(col string) string {
	return fmt.Sprintf("toUInt32(toIPv4OrZero(toString(%s)))", col)
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
			parts = append(parts, fmt.Sprintf("(%s BETWEEN ? AND ?)", ipv4AsUInt32SQL(col)))
			args = append(args, n.Start, n.End)
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "AND (" + strings.Join(parts, " OR ") + ")", args
}

func excludeSrcSQL(ips []string) (string, []any) {
	clean := make([]string, 0, len(ips))
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		clean = append(clean, ip)
	}
	if len(clean) == 0 {
		return "", nil
	}
	parts := make([]string, len(clean))
	args := make([]any, len(clean))
	for i, ip := range clean {
		parts[i] = "?"
		args[i] = ip
	}
	return "AND toString(src_ip) NOT IN (" + strings.Join(parts, ",") + ")", args
}

func privateSrcSQL(includePrivate bool) string {
	if includePrivate {
		return ""
	}
	// Normalize String or IPv4 columns — bare src_ip >= toIPv4(...) fails with CH code 386
	// when traffic_logs.src_ip is still String.
	ip := "toIPv4OrZero(toString(src_ip))"
	return fmt.Sprintf(`
		AND NOT (
			(%[1]s >= toIPv4('10.0.0.0') AND %[1]s <= toIPv4('10.255.255.255'))
			OR (%[1]s >= toIPv4('172.16.0.0') AND %[1]s <= toIPv4('172.31.255.255'))
			OR (%[1]s >= toIPv4('192.168.0.0') AND %[1]s <= toIPv4('192.168.255.255'))
			OR (%[1]s >= toIPv4('127.0.0.0') AND %[1]s <= toIPv4('127.255.255.255'))
			OR (%[1]s >= toIPv4('169.254.0.0') AND %[1]s <= toIPv4('169.254.255.255'))
		)`, ip)
}
