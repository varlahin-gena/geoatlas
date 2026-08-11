package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
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

// backupTablesSQL — CH 25: перед каждым объектом нужен keyword TABLE
// (`BACKUP TABLE a, TABLE b TO …`), не `BACKUP TABLE a, b`.
func backupTablesSQL(tables []string, name string) string {
	parts := make([]string, 0, len(tables))
	for _, t := range tables {
		parts = append(parts, "TABLE "+t)
	}
	return fmt.Sprintf("BACKUP %s TO Disk('backups', '%s')", strings.Join(parts, ", "), name)
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
