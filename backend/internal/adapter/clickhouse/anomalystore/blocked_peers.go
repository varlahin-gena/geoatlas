package anomalystore

import (
	"context"
	"fmt"
	"time"

	"geoatlas/internal/adapter/clickhouse/query"
	"geoatlas/internal/model"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
)

func (r *Repository) BlockedPeers(ctx context.Context, start, end time.Time, net usecaseanomaly.IPRange, limit int) ([]usecaseanomaly.BlockedPeer, []usecaseanomaly.BlockedPeer, error) {
	if limit < 1 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	src, err := r.blockedPeerSide(ctx, start, end, net, "src_ip", limit)
	if err != nil {
		return nil, nil, err
	}
	dst, err := r.blockedPeerSide(ctx, start, end, net, "dst_ip", limit)
	if err != nil {
		return nil, nil, err
	}
	return src, dst, nil
}

func (r *Repository) blockedPeerSide(ctx context.Context, start, end time.Time, net usecaseanomaly.IPRange, col string, limit int) ([]usecaseanomaly.BlockedPeer, error) {
	touch, touchArgs := touchNetsSQL([]usecaseanomaly.IPRange{net})
	q := fmt.Sprintf(`
		SELECT toString(%s) AS ip, count() AS n
		FROM traffic_logs
		WHERE timestamp >= ? AND timestamp < ?
		  AND lower(action) IN (%s)
		  %s
		GROUP BY %s
		ORDER BY n DESC
		LIMIT %d
		%s
	`, col, model.BlockedInClause(), touch, col, limit, query.AggSettings())
	args := append([]any{start, end}, touchArgs...)
	rows, err := r.ch.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []usecaseanomaly.BlockedPeer
	for rows.Next() {
		var p usecaseanomaly.BlockedPeer
		if err := rows.Scan(&p.IP, &p.Events); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
