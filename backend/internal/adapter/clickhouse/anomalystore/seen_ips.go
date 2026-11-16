package anomalystore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"geoatlas/internal/adapter/clickhouse/query"
)

func (r *Repository) SeenIPs(ctx context.Context, ips []string, from, to time.Time) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if r == nil || r.ch == nil || len(ips) == 0 {
		return out, nil
	}
	clean := make([]string, 0, len(ips))
	seen := map[string]struct{}{}
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		clean = append(clean, ip)
	}
	if len(clean) == 0 {
		return out, nil
	}
	ph := make([]string, len(clean))
	args := make([]any, 0, len(clean)+2)
	args = append(args, from, to)
	for i, ip := range clean {
		ph[i] = "?"
		args = append(args, ip)
	}
	inList := strings.Join(ph, ",")
	q := fmt.Sprintf(`
		SELECT DISTINCT ip FROM (
			SELECT toString(src_ip) AS ip
			FROM traffic_edges_hourly
			WHERE hour >= ? AND hour < ? AND toString(src_ip) IN (%s)
			UNION ALL
			SELECT toString(dst_ip) AS ip
			FROM traffic_edges_hourly
			WHERE hour >= ? AND hour < ? AND toString(dst_ip) IN (%s)
		)
		%s
	`, inList, inList, query.AggSettings())
	args = append(args, from, to)
	for _, ip := range clean {
		args = append(args, ip)
	}
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		if !shouldFallbackToLogs(ctx, err) {
			return nil, err
		}
		q2 := fmt.Sprintf(`
			SELECT DISTINCT ip FROM (
				SELECT toString(src_ip) AS ip
				FROM traffic_logs
				WHERE timestamp >= ? AND timestamp < ? AND toString(src_ip) IN (%s)
				UNION ALL
				SELECT toString(dst_ip) AS ip
				FROM traffic_logs
				WHERE timestamp >= ? AND timestamp < ? AND toString(dst_ip) IN (%s)
			)
			%s
		`, inList, inList, query.AggSettings())
		rows, err = r.ch.Query(ctx, q2, args...)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, err
		}
		if ip != "" {
			out[ip] = struct{}{}
		}
	}
	return out, rows.Err()
}
