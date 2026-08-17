package backupstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/clickhouse/query"
)

// BackupRunner — native BACKUP TABLE … TO Disk('backups', name).
type BackupRunner struct {
	ch clickhouse.Conn
}

func NewBackupRunner(ch clickhouse.Conn) *BackupRunner {
	return &BackupRunner{ch: ch}
}

func (r *BackupRunner) TableExists(ctx context.Context, name string) (bool, error) {
	if r == nil || r.ch == nil {
		return false, fmt.Errorf("clickhouse not configured")
	}
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var n uint64
	err := r.ch.QueryRow(qctx, `
		SELECT count()
		FROM system.tables
		WHERE database = currentDatabase() AND name = ?
	`, name).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *BackupRunner) BackupTables(ctx context.Context, name string, tables []string) error {
	if r == nil || r.ch == nil {
		return fmt.Errorf("clickhouse not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "'\\\"/;") {
		return fmt.Errorf("invalid backup name")
	}
	safe := make([]string, 0, len(tables))
	for _, t := range tables {
		t = strings.TrimSpace(t)
		if t == "" || !isSafeIdent(t) {
			return fmt.Errorf("invalid table name %q", t)
		}
		safe = append(safe, t)
	}
	if len(safe) == 0 {
		return fmt.Errorf("no tables")
	}
	sql := backupTablesSQL(safe, name)
	qctx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()
	if err := r.ch.Exec(qctx, sql); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	return nil
}

// DropTables удаляет shadow-таблицы бэкапа.
func (r *BackupRunner) DropTables(ctx context.Context, tables []string) error {
	if r == nil || r.ch == nil {
		return fmt.Errorf("clickhouse not configured")
	}
	qctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	for _, t := range tables {
		t = strings.TrimSpace(t)
		if t == "" || !isSafeIdent(t) {
			return fmt.Errorf("invalid table name %q", t)
		}
		if err := r.ch.Exec(qctx, "DROP TABLE IF EXISTS "+t); err != nil {
			return fmt.Errorf("drop %s: %w", t, err)
		}
	}
	return nil
}

// RestoreTablesAs восстанавливает src из бэкапа в dest (shadow). Dest предварительно DROP.
// pairs: [srcLive, destShadow].
func (r *BackupRunner) RestoreTablesAs(ctx context.Context, name string, pairs [][2]string) error {
	if r == nil || r.ch == nil {
		return fmt.Errorf("clickhouse not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "'\\\"/;") {
		return fmt.Errorf("invalid backup name")
	}
	qctx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()
	restored := 0
	for _, p := range pairs {
		src, dest := strings.TrimSpace(p[0]), strings.TrimSpace(p[1])
		if src == "" || dest == "" || !isSafeIdent(src) || !isSafeIdent(dest) {
			return fmt.Errorf("invalid table pair %q → %q", src, dest)
		}
		if err := r.ch.Exec(qctx, "DROP TABLE IF EXISTS "+dest); err != nil {
			return fmt.Errorf("drop %s: %w", dest, err)
		}
		sql := restoreTableAsSQL(src, dest, name)
		if err := r.ch.Exec(qctx, sql); err != nil {
			if isMissingBackupObject(err) {
				continue
			}
			return fmt.Errorf("restore %s as %s: %w", src, dest, err)
		}
		restored++
	}
	if restored == 0 {
		return fmt.Errorf("restore: nothing restored from %s", name)
	}
	return nil
}

// RestoreMapShadow — RESTORE live-таблиц карты в nm_bak_* (имена из query.MapShadowPairs).
func (r *BackupRunner) RestoreMapShadow(ctx context.Context, name string) error {
	return r.RestoreTablesAs(ctx, name, query.MapShadowPairs())
}

// DropMapShadow удаляет shadow-таблицы карты (query.MapShadowNames).
func (r *BackupRunner) DropMapShadow(ctx context.Context) error {
	return r.DropTables(ctx, query.MapShadowNames())
}

// backupTablesSQL — CH 25: перед каждым объектом нужен keyword TABLE
// (`BACKUP TABLE a, TABLE b TO …`), не `BACKUP TABLE a, b`.
func backupTablesSQL(tables []string, name string) string {
	parts := make([]string, 0, len(tables))
	for _, t := range tables {
		parts = append(parts, "TABLE "+t)
	}
	return fmt.Sprintf("BACKUP %s TO Disk('backups', '%s')", strings.Join(parts, ", "), name)
}

func restoreTableAsSQL(src, dest, name string) string {
	return fmt.Sprintf(
		"RESTORE TABLE %s AS %s FROM Disk('backups', '%s')",
		src, dest, name,
	)
}

func isMissingBackupObject(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"not found",
		"doesn't exist",
		"does not exist",
		"unknown_table",
		"no_such",
		"cannot find",
		"backup doesn't",
		"backup does not",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

func isSafeIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}
