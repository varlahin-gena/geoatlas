package geostore

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/model"
	"network_monitor/internal/usecase/geo"
)

func (r *GeoRepository) ListEnterpriseNets(ctx context.Context) ([]model.EnterpriseNet, error) {
	return listEnterpriseNets(ctx, r.apiConn())
}

func (r *GeoRepository) UpsertEnterpriseNet(ctx context.Context, net model.EnterpriseNet) error {
	return upsertEnterpriseNet(ctx, r.writeCH, net)
}

func (r *GeoRepository) DeleteEnterpriseNet(ctx context.Context, startIP, endIP uint32) error {
	return deleteEnterpriseNet(ctx, r.writeCH, startIP, endIP)
}

func (r *GeoRepository) CountEnterpriseNets(ctx context.Context) (int, error) {
	return countEnterpriseNets(ctx, r.apiConn())
}

var _ geo.EnterpriseNetStore = (*GeoRepository)(nil)

func listEnterpriseNets(ctx context.Context, ch clickhouse.Conn) ([]model.EnterpriseNet, error) {
	if ch == nil {
		return nil, fmt.Errorf("clickhouse conn is nil")
	}
	rows, err := ch.Query(ctx, `
		SELECT start_ip, end_ip, network, label, country, region, city, created_at
		FROM enterprise_nets FINAL
		WHERE is_deleted = 0
		ORDER BY start_ip, end_ip
		LIMIT ?
	`, geo.MaxEnterpriseNets)
	if err != nil {
		return nil, fmt.Errorf("list enterprise_nets: %w", err)
	}
	defer rows.Close()
	out := make([]model.EnterpriseNet, 0)
	for rows.Next() {
		var n model.EnterpriseNet
		if err := rows.Scan(&n.StartIP, &n.EndIP, &n.Network, &n.Label, &n.Country, &n.Region, &n.City, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func countEnterpriseNets(ctx context.Context, ch clickhouse.Conn) (int, error) {
	if ch == nil {
		return 0, fmt.Errorf("clickhouse conn is nil")
	}
	var n uint64
	if err := ch.QueryRow(ctx, `
		SELECT count() FROM enterprise_nets FINAL WHERE is_deleted = 0
	`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count enterprise_nets: %w", err)
	}
	return int(n), nil
}

func upsertEnterpriseNet(ctx context.Context, ch clickhouse.Conn, net model.EnterpriseNet) error {
	if ch == nil {
		return fmt.Errorf("clickhouse conn is nil")
	}
	if net.CreatedAt.IsZero() {
		net.CreatedAt = time.Now().UTC()
	}
	ver := uint64(time.Now().UTC().UnixNano())
	return ch.Exec(ctx, `
		INSERT INTO enterprise_nets
		(start_ip, end_ip, network, label, country, region, city, created_at, ver, is_deleted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
	`, net.StartIP, net.EndIP, net.Network, net.Label, net.Country, net.Region, net.City, net.CreatedAt, ver)
}

func deleteEnterpriseNet(ctx context.Context, ch clickhouse.Conn, startIP, endIP uint32) error {
	if ch == nil {
		return fmt.Errorf("clickhouse conn is nil")
	}
	now := time.Now().UTC()
	ver := uint64(now.UnixNano())
	return ch.Exec(ctx, `
		INSERT INTO enterprise_nets
		(start_ip, end_ip, network, label, country, region, city, created_at, ver, is_deleted)
		VALUES (?, ?, '', '', '', '', '', ?, ?, 1)
	`, startIP, endIP, now, ver)
}
