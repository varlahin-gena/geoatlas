package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"network_monitor/internal/model"
)

const geoRangesInsertBatchSize = 25_000

// Сериализует полную замену geo_ranges (staging table одна на процесс).
var replaceGeoRangesMu sync.Mutex

// LoadGeoRanges читает все диапазоны из geo_ranges (ORDER BY start_ip, end_ip).
func LoadGeoRanges(ctx context.Context, ch clickhouse.Conn) ([]model.GeoRange, error) {
	if ch == nil {
		return nil, fmt.Errorf("clickhouse conn is nil")
	}
	rows, err := ch.Query(ctx, `
		SELECT start_ip, end_ip, country, region, city, lat, lon
		FROM geo_ranges
		ORDER BY start_ip, end_ip
	`)
	if err != nil {
		return nil, fmt.Errorf("query geo_ranges: %w", err)
	}
	defer rows.Close()
	return scanGeoRangeRows(rows)
}

// CountGeoRanges — число строк в geo_ranges.
func CountGeoRanges(ctx context.Context, ch clickhouse.Conn) (int, error) {
	if ch == nil {
		return 0, fmt.Errorf("clickhouse conn is nil")
	}
	var n uint64
	if err := ch.QueryRow(ctx, `SELECT count() FROM geo_ranges`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count geo_ranges: %w", err)
	}
	return int(n), nil
}

// FindGeoRangeByIP — один покрывающий диапазон из CH (без полной загрузки таблицы).
func FindGeoRangeByIP(ctx context.Context, ch clickhouse.Conn, ipStr string) (model.GeoRange, bool, error) {
	if ch == nil {
		return model.GeoRange{}, false, fmt.Errorf("clickhouse conn is nil")
	}
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil || ip.To4() == nil {
		return model.GeoRange{}, false, nil
	}
	b := ip.To4()
	v := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	row := ch.QueryRow(ctx, `
		SELECT start_ip, end_ip, country, region, city, lat, lon
		FROM geo_ranges
		WHERE start_ip <= ? AND end_ip >= ?
		ORDER BY start_ip DESC
		LIMIT 1
	`, v, v)
	var g model.GeoRange
	if err := row.Scan(&g.StartIP, &g.EndIP, &g.Country, &g.Region, &g.City, &g.Lat, &g.Lon); err != nil {
		if isNoRows(err) {
			return model.GeoRange{}, false, nil
		}
		return model.GeoRange{}, false, err
	}
	return g, true, nil
}

// ListGeoRangesPage — страница диапазонов из CH (LIMIT + optional текстовый фильтр).
func ListGeoRangesPage(ctx context.Context, ch clickhouse.Conn, limit int, q string) (items []model.GeoRange, total, filtered int, truncated bool, err error) {
	if ch == nil {
		return nil, 0, 0, false, fmt.Errorf("clickhouse conn is nil")
	}
	if limit <= 0 {
		limit = 2000
	}
	total, err = CountGeoRanges(ctx, ch)
	if err != nil {
		return nil, 0, 0, false, err
	}
	q = strings.TrimSpace(q)
	if q == "" {
		rows, qerr := ch.Query(ctx, `
			SELECT start_ip, end_ip, country, region, city, lat, lon
			FROM geo_ranges
			ORDER BY start_ip, end_ip
			LIMIT ?
		`, limit)
		if qerr != nil {
			return nil, 0, 0, false, qerr
		}
		defer rows.Close()
		items, err = scanGeoRangeRows(rows)
		if err != nil {
			return nil, 0, 0, false, err
		}
		return items, total, total, total > limit, nil
	}

	var filteredU uint64
	if err := ch.QueryRow(ctx, `
		SELECT count() FROM geo_ranges
		WHERE positionCaseInsensitive(country, ?) > 0
		   OR positionCaseInsensitive(region, ?) > 0
		   OR positionCaseInsensitive(city, ?) > 0
		   OR positionCaseInsensitive(IPv4NumToString(toUInt32(start_ip)), ?) > 0
		   OR positionCaseInsensitive(IPv4NumToString(toUInt32(end_ip)), ?) > 0
	`, q, q, q, q, q).Scan(&filteredU); err != nil {
		return nil, 0, 0, false, err
	}
	filtered = int(filteredU)
	rows, qerr := ch.Query(ctx, `
		SELECT start_ip, end_ip, country, region, city, lat, lon
		FROM geo_ranges
		WHERE positionCaseInsensitive(country, ?) > 0
		   OR positionCaseInsensitive(region, ?) > 0
		   OR positionCaseInsensitive(city, ?) > 0
		   OR positionCaseInsensitive(IPv4NumToString(toUInt32(start_ip)), ?) > 0
		   OR positionCaseInsensitive(IPv4NumToString(toUInt32(end_ip)), ?) > 0
		ORDER BY start_ip, end_ip
		LIMIT ?
	`, q, q, q, q, q, limit)
	if qerr != nil {
		return nil, 0, 0, false, qerr
	}
	defer rows.Close()
	items, err = scanGeoRangeRows(rows)
	if err != nil {
		return nil, 0, 0, false, err
	}
	return items, total, filtered, filtered > limit, nil
}

func scanGeoRangeRows(rows driver.Rows) ([]model.GeoRange, error) {
	var ranges []model.GeoRange
	for rows.Next() {
		var g model.GeoRange
		if err := rows.Scan(&g.StartIP, &g.EndIP, &g.Country, &g.Region, &g.City, &g.Lat, &g.Lon); err != nil {
			return nil, err
		}
		ranges = append(ranges, g)
	}
	return ranges, rows.Err()
}

func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no rows") || strings.Contains(msg, "empty result") || msg == "eof"
}

// ReplaceGeoRanges атомарно заменяет geo_ranges через staging + EXCHANGE TABLES.
// При сбое INSERT/validate исходная таблица не трогается.
// Парсинг CSV — в geoip.ReadCSV; этот слой только пишет в ClickHouse.
func ReplaceGeoRanges(ctx context.Context, ch clickhouse.Conn, ranges []model.GeoRange) (int, error) {
	if len(ranges) == 0 {
		return 0, fmt.Errorf("no geo ranges to insert")
	}
	if ch == nil {
		return 0, fmt.Errorf("clickhouse conn is nil")
	}

	replaceGeoRangesMu.Lock()
	defer replaceGeoRangesMu.Unlock()

	const staging = "geo_ranges__staging"

	// Хвост от прошлого краша: убрать перед CREATE.
	_ = ch.Exec(ctx, "DROP TABLE IF EXISTS "+staging)

	if err := ch.Exec(ctx, "CREATE TABLE "+staging+" AS geo_ranges"); err != nil {
		return 0, fmt.Errorf("create %s: %w", staging, err)
	}
	dropStaging := func(ctx context.Context) {
		// WithoutCancel: cleanup должен пройти даже если request ctx уже отменён.
		dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_ = ch.Exec(dctx, "DROP TABLE IF EXISTS "+staging)
	}

	count, err := insertGeoRangesInto(ctx, ch, staging, ranges)
	if err != nil {
		dropStaging(ctx)
		return count, err
	}

	var got uint64
	if err := ch.QueryRow(ctx, "SELECT count() FROM "+staging).Scan(&got); err != nil {
		dropStaging(ctx)
		return count, fmt.Errorf("count staging: %w", err)
	}
	if int(got) != count {
		dropStaging(ctx)
		return count, fmt.Errorf("staging row count mismatch: got %d want %d", got, count)
	}

	if err := ch.Exec(ctx, "EXCHANGE TABLES geo_ranges AND "+staging); err != nil {
		dropStaging(ctx)
		return count, fmt.Errorf("exchange geo_ranges: %w", err)
	}
	// После EXCHANGE в staging — старые данные.
	dropStaging(ctx)
	return count, nil
}

// TruncateGeoRanges очищает таблицу geo_ranges (полная замена через UI/API).
func TruncateGeoRanges(ctx context.Context, ch clickhouse.Conn) error {
	if ch == nil {
		return fmt.Errorf("clickhouse conn is nil")
	}
	replaceGeoRangesMu.Lock()
	defer replaceGeoRangesMu.Unlock()
	if err := ch.Exec(ctx, "TRUNCATE TABLE IF EXISTS geo_ranges"); err != nil {
		return fmt.Errorf("truncate geo_ranges: %w", err)
	}
	_ = ch.Exec(ctx, "DROP TABLE IF EXISTS geo_ranges__staging")
	return nil
}

func insertGeoRangesInto(ctx context.Context, ch clickhouse.Conn, table string, ranges []model.GeoRange) (int, error) {
	var (
		batch driver.Batch
		count int
	)

	flush := func() error {
		if batch == nil {
			return nil
		}
		if err := batch.Send(); err != nil {
			return err
		}
		batch = nil
		return nil
	}

	insertSQL := fmt.Sprintf(`
			INSERT INTO %s (start_ip, end_ip, country, region, city, lat, lon)
		`, table)

	newBatch := func() error {
		var err error
		batch, err = ch.PrepareBatch(ctx, insertSQL)
		return err
	}

	for _, g := range ranges {
		if err := ctx.Err(); err != nil {
			if batch != nil {
				_ = batch.Abort()
			}
			return count, err
		}
		if batch == nil {
			if err := newBatch(); err != nil {
				return count, err
			}
		}
		if err := batch.Append(g.StartIP, g.EndIP, g.Country, g.Region, g.City, g.Lat, g.Lon); err != nil {
			_ = batch.Abort()
			return count, err
		}
		count++
		if count%geoRangesInsertBatchSize == 0 {
			if err := flush(); err != nil {
				return count, err
			}
		}
	}

	if err := flush(); err != nil {
		return count, err
	}
	return count, nil
}
