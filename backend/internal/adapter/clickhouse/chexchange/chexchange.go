// Package chexchange — атомарная замена ClickHouse-таблиц через EXCHANGE TABLES.
// Используется migrate (schema rebuild) и geostore/repstore (staging replace).
package chexchange

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const dropCleanupTimeout = 30 * time.Second

// Exec выполняет одну DDL/DML строку (migrate оборачивает в timeout).
type Exec func(ctx context.Context, query string) error

// Conn — минимум для staging-replace (geostore/repstore).
type Conn interface {
	Exec(ctx context.Context, query string, args ...any) error
}

// SwapAndDrop атомарно меняет live и other местами, затем дропает other
// (там оказывается прежнее содержимое live). При ошибке EXCHANGE — best-effort DROP other.
func SwapAndDrop(ctx context.Context, exec Exec, live, other string) error {
	live, other = strings.TrimSpace(live), strings.TrimSpace(other)
	if live == "" || other == "" {
		return fmt.Errorf("chexchange: empty table name")
	}
	if exec == nil {
		return fmt.Errorf("chexchange: nil exec")
	}
	if err := exec(ctx, "EXCHANGE TABLES "+live+" AND "+other); err != nil {
		dropBestEffort(ctx, exec, other)
		return fmt.Errorf("exchange %s: %w", live, err)
	}
	dropBestEffort(ctx, exec, other)
	return nil
}

// RebuildViaNext пересоздаёт live пустой таблицей с новой схемой:
// DROP next → CREATE next → EXCHANGE → DROP next.
func RebuildViaNext(ctx context.Context, exec Exec, live, next, createNextSQL string) error {
	live, next = strings.TrimSpace(live), strings.TrimSpace(next)
	if live == "" || next == "" {
		return fmt.Errorf("chexchange: empty table name")
	}
	if strings.TrimSpace(createNextSQL) == "" {
		return fmt.Errorf("chexchange: empty create SQL")
	}
	if exec == nil {
		return fmt.Errorf("chexchange: nil exec")
	}
	_ = exec(ctx, "DROP TABLE IF EXISTS "+next)
	if err := exec(ctx, createNextSQL); err != nil {
		dropBestEffort(ctx, exec, next)
		return fmt.Errorf("create %s: %w", next, err)
	}
	return SwapAndDrop(ctx, exec, live, next)
}

// ReplaceViaStaging атомарно подменяет live через staging-копию схемы:
// DROP staging → CREATE staging AS live → fill → EXCHANGE → DROP staging.
// fill пишет данные в staging; при ошибке до успешного EXCHANGE staging снимается.
func ReplaceViaStaging(ctx context.Context, ch Conn, live, staging string, fill func(ctx context.Context) error) error {
	live, staging = strings.TrimSpace(live), strings.TrimSpace(staging)
	if live == "" || staging == "" {
		return fmt.Errorf("chexchange: empty table name")
	}
	if ch == nil {
		return fmt.Errorf("chexchange: nil conn")
	}
	if fill == nil {
		return fmt.Errorf("chexchange: nil fill")
	}

	_ = ch.Exec(ctx, "DROP TABLE IF EXISTS "+staging)
	if err := ch.Exec(ctx, "CREATE TABLE "+staging+" AS "+live); err != nil {
		return fmt.Errorf("create %s: %w", staging, err)
	}

	dropStaging := func() {
		dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dropCleanupTimeout)
		defer cancel()
		_ = ch.Exec(dctx, "DROP TABLE IF EXISTS "+staging)
	}

	if err := fill(ctx); err != nil {
		dropStaging()
		return err
	}

	if err := ch.Exec(ctx, "EXCHANGE TABLES "+live+" AND "+staging); err != nil {
		dropStaging()
		return fmt.Errorf("exchange %s: %w", live, err)
	}
	dropStaging()
	return nil
}

func dropBestEffort(parent context.Context, exec Exec, table string) {
	dctx, cancel := context.WithTimeout(context.WithoutCancel(parent), dropCleanupTimeout)
	defer cancel()
	_ = exec(dctx, "DROP TABLE IF EXISTS "+table)
}
