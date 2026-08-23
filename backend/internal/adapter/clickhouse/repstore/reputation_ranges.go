package repstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"geoatlas/internal/model"
)

const reputationRangesInsertBatchSize = 25_000

var replaceReputationRangesMu sync.Mutex

// LoadReputationRanges читает все диапазоны.
func LoadReputationRanges(ctx context.Context, ch clickhouse.Conn) ([]model.ReputationRange, error) {
	if ch == nil {
		return nil, fmt.Errorf("clickhouse conn is nil")
	}
	rows, err := ch.Query(ctx, `
		SELECT list_name, category, start_ip, end_ip, source, updated_at
		FROM reputation_ranges
		ORDER BY list_name, start_ip, end_ip
	`)
	if err != nil {
		return nil, fmt.Errorf("query reputation_ranges: %w", err)
	}
	defer rows.Close()
	return scanReputationRangeRows(rows)
}

func scanReputationRangeRows(rows driver.Rows) ([]model.ReputationRange, error) {
	out := make([]model.ReputationRange, 0, 4096)
	for rows.Next() {
		var r model.ReputationRange
		if err := rows.Scan(&r.ListName, &r.Category, &r.StartIP, &r.EndIP, &r.Source, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListReputationMeta — агрегаты по list_name.
func ListReputationMeta(ctx context.Context, ch clickhouse.Conn) ([]model.ReputationListMeta, error) {
	if ch == nil {
		return nil, fmt.Errorf("clickhouse conn is nil")
	}
	rows, err := ch.Query(ctx, `
		SELECT
			list_name,
			any(category) AS category,
			count() AS cnt,
			any(source) AS source,
			max(updated_at) AS updated_at
		FROM reputation_ranges
		GROUP BY list_name
		ORDER BY list_name
	`)
	if err != nil {
		return nil, fmt.Errorf("list reputation meta: %w", err)
	}
	defer rows.Close()
	var out []model.ReputationListMeta
	for rows.Next() {
		var m model.ReputationListMeta
		var cnt uint64
		if err := rows.Scan(&m.Name, &m.Category, &cnt, &m.Source, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Count = int(cnt)
		out = append(out, m)
	}
	return out, rows.Err()
}

// ReplaceReputationRanges атомарно заменяет всю таблицу.
func ReplaceReputationRanges(ctx context.Context, ch clickhouse.Conn, ranges []model.ReputationRange) (int, error) {
	if ch == nil {
		return 0, fmt.Errorf("clickhouse conn is nil")
	}
	replaceReputationRangesMu.Lock()
	defer replaceReputationRangesMu.Unlock()

	const staging = "reputation_ranges__staging"
	_ = ch.Exec(ctx, "DROP TABLE IF EXISTS "+staging)
	if err := ch.Exec(ctx, "CREATE TABLE "+staging+" AS reputation_ranges"); err != nil {
		return 0, fmt.Errorf("create %s: %w", staging, err)
	}
	dropStaging := func(ctx context.Context) {
		dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_ = ch.Exec(dctx, "DROP TABLE IF EXISTS "+staging)
	}

	if len(ranges) == 0 {
		// Пустая замена: exchange пустой staging.
		if err := ch.Exec(ctx, "EXCHANGE TABLES reputation_ranges AND "+staging); err != nil {
			dropStaging(ctx)
			return 0, fmt.Errorf("exchange reputation_ranges: %w", err)
		}
		dropStaging(ctx)
		return 0, nil
	}

	count, err := insertReputationRangesInto(ctx, ch, staging, ranges)
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
	if err := ch.Exec(ctx, "EXCHANGE TABLES reputation_ranges AND "+staging); err != nil {
		dropStaging(ctx)
		return count, fmt.Errorf("exchange reputation_ranges: %w", err)
	}
	dropStaging(ctx)
	return count, nil
}

func insertReputationRangesInto(ctx context.Context, ch clickhouse.Conn, table string, ranges []model.ReputationRange) (int, error) {
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
		INSERT INTO %s (list_name, category, start_ip, end_ip, source, updated_at)
	`, table)
	newBatch := func() error {
		var err error
		batch, err = ch.PrepareBatch(ctx, insertSQL)
		return err
	}
	for _, r := range ranges {
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
		if err := batch.Append(r.ListName, r.Category, r.StartIP, r.EndIP, r.Source, r.UpdatedAt); err != nil {
			_ = batch.Abort()
			return count, err
		}
		count++
		if count%reputationRangesInsertBatchSize == 0 {
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
