package anomalystore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	usecaseanomaly "geoatlas/internal/usecase/anomaly"
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
			if(a.fingerprint = '', 0, 1) AS acked,
			ifNull(a.ack_by, '') AS ack_by,
			ifNull(asg.assigned_to, '') AS assigned_to
		FROM anomaly_events AS e
		LEFT JOIN (
			SELECT fingerprint, argMax(ack_by, ack_at) AS ack_by
			FROM anomaly_acks
			GROUP BY fingerprint
		) AS a ON e.fingerprint = a.fingerprint
		LEFT JOIN (
			SELECT fingerprint, argMax(assigned_to, assigned_at) AS assigned_to
			FROM anomaly_assignments
			GROUP BY fingerprint
		) AS asg ON e.fingerprint = asg.fingerprint
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
			&e.EventCount, &e.Fingerprint, &e.ExpiresAt, &acked, &e.AckBy, &e.AssignedTo,
		); err != nil {
			return nil, err
		}
		e.Acknowledged = acked != 0
		e.CodeLabel = usecaseanomaly.CodeHumanLabel(e.Code)
		e.SrcIP = displayIP(e.SrcIP)
		e.DstIP = displayIP(e.DstIP)
		if detail != "" && detail != "{}" {
			_ = json.Unmarshal([]byte(detail), &e.Detail)
		}
		e.Map = usecaseanomaly.MapLinkFor(e)
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

	// Special-case: new_country_dst repeat guard should not depend on anomaly_events.suppression_key
	// being filled (e.g. after schema migration/backfill). We can use dst_country directly.
	if code == usecaseanomaly.CodeNewCountryDst {
		countries := make([]string, 0, len(keys))
		for _, k := range keys {
			s := strings.TrimSpace(string(k))
			const needle = "|country|"
			if idx := strings.LastIndex(s, needle); idx >= 0 {
				c := strings.TrimSpace(s[idx+len(needle):])
				if c != "" {
					countries = append(countries, c)
				}
			}
		}
		if len(countries) == 0 {
			return out, nil
		}
		placeholders := make([]string, 0, len(countries))
		args := make([]any, 0, len(countries)+2)
		args = append(args, code, since.UTC())
		for _, c := range countries {
			placeholders = append(placeholders, "?")
			args = append(args, c)
		}
		q := `SELECT dst_country FROM anomaly_events
			WHERE code = ? AND detected_at >= ? AND dst_country IN (` + strings.Join(placeholders, ",") + `)
			GROUP BY dst_country`
		rows, err := r.ch.Query(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				return nil, err
			}
			if c != "" {
				k := usecaseanomaly.SuppressionKey(code + "|country|" + c)
				out[k] = struct{}{}
			}
		}
		return out, rows.Err()
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
	// Compute suppression key from stored fields if anomaly_events.suppression_key is empty
	// (e.g. events created before migration/backfill).
	var (
		code   string
		key    string
		src    string
		dst    string
		city   string
		device string
	)
	err := r.ch.QueryRow(ctx, `
		SELECT
			code,
			suppression_key,
			toString(src_ip),
			toString(dst_ip),
			dst_country,
			device
		FROM anomaly_events
		WHERE fingerprint = ?
		ORDER BY detected_at DESC
		LIMIT 1
	`, fingerprint).Scan(&code, &key, &src, &dst, &city, &device)
	if err != nil {
		return err
	}

	trimKey := strings.TrimSpace(key)
	if trimKey == "" {
		switch code {
		case usecaseanomaly.CodeNewCountryDst:
			trimKey = code + "|country|" + strings.TrimSpace(city)
		case usecaseanomaly.CodePortScan, usecaseanomaly.CodeHorizontalScan:
			trimKey = code + "|src|" + strings.TrimSpace(src)
		case usecaseanomaly.CodeRepNewDst:
			trimKey = code + "|pair|" + strings.TrimSpace(src) + "|" + strings.TrimSpace(dst)
		case usecaseanomaly.CodeBlockedSurge:
			if d := strings.TrimSpace(device); d != "" {
				trimKey = code + "|net|" + d
			} else {
				trimKey = code + "|global"
			}
		}
	}
	if strings.TrimSpace(trimKey) == "" {
		return nil
	}
	return r.ch.Exec(ctx, `
		INSERT INTO anomaly_suppressions
		(suppression_key, code, source_fingerprint, suppressed_at, suppressed_until, suppressed_by)
		VALUES (?, ?, ?, ?, ?, ?)
	`, trimKey, code, fingerprint, now, now.Add(suppressFor), by)
}

func (r *Repository) currentAssignee(ctx context.Context, fingerprint string) (string, error) {
	var to string
	err := r.ch.QueryRow(ctx, `
		SELECT argMax(assigned_to, assigned_at)
		FROM anomaly_assignments
		WHERE fingerprint = ?
	`, fingerprint).Scan(&to)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(to), nil
}

func (r *Repository) Assign(ctx context.Context, fingerprint, assignedTo, by string) error {
	if r == nil || r.ch == nil {
		return fmt.Errorf("clickhouse not configured")
	}
	now := time.Now().UTC()
	return r.ch.Exec(ctx, `
		INSERT INTO anomaly_assignments (fingerprint, assigned_to, assigned_at, assigned_by)
		VALUES (?, ?, ?, ?)
	`, fingerprint, assignedTo, now, by)
}

func (r *Repository) AssignIfEmpty(ctx context.Context, fingerprint, assignedTo, by string) error {
	if r == nil || r.ch == nil {
		return fmt.Errorf("clickhouse not configured")
	}
	cur, err := r.currentAssignee(ctx, fingerprint)
	if err != nil {
		// Empty table / no rows — treat as unassigned.
		cur = ""
	}
	if cur != "" {
		return nil
	}
	return r.Assign(ctx, fingerprint, assignedTo, by)
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

