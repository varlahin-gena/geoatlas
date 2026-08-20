package auditstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"

	usecaseaudit "network_monitor/internal/usecase/auditlog"
)

type Repository struct {
	ch clickhouse.Conn
}

func New(ch clickhouse.Conn) *Repository { return &Repository{ch: ch} }

func normalizeListLimit(limit int) int {
	if limit < 1 || limit > 200 {
		return 100
	}
	return limit
}

func (r *Repository) WriteDR(ctx context.Context, e usecaseaudit.DREvent) error {
	if r == nil || r.ch == nil {
		return nil
	}
	meta := "{}"
	if e.Meta != nil {
		if b, err := json.Marshal(e.Meta); err == nil {
			meta = string(b)
		}
	}
	return r.ch.Exec(ctx, `
		INSERT INTO dr_events (timestamp, actor, action, target, status, message, meta)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, e.Timestamp, e.Actor, e.Action, e.Target, e.Status, e.Message, meta)
}

func (r *Repository) WriteAudit(ctx context.Context, e usecaseaudit.AuditEvent) error {
	if r == nil || r.ch == nil {
		return nil
	}
	details := "{}"
	if e.Details != nil {
		if b, err := json.Marshal(e.Details); err == nil {
			details = string(b)
		}
	}
	return r.ch.Exec(ctx, `
		INSERT INTO audit_events (timestamp, actor, action, resource_type, resource_id, result, ip, details)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, e.Timestamp, e.Actor, e.Action, e.ResourceType, e.ResourceID, e.Result, e.IP, details)
}

func (r *Repository) ListDR(ctx context.Context, q usecaseaudit.DRQuery) ([]usecaseaudit.DREvent, error) {
	if r == nil || r.ch == nil {
		return nil, fmt.Errorf("clickhouse not configured")
	}
	limit := normalizeListLimit(q.Limit)
	var sb strings.Builder
	args := []any{q.Since}
	sb.WriteString(`SELECT timestamp, actor, action, target, status, message, meta FROM dr_events WHERE timestamp >= ?`)
	if v := strings.TrimSpace(q.Action); v != "" {
		sb.WriteString(` AND action = ?`)
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		sb.WriteString(` AND status = ?`)
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Actor); v != "" {
		sb.WriteString(` AND actor = ?`)
		args = append(args, v)
	}
	sb.WriteString(` ORDER BY timestamp DESC LIMIT ?`)
	args = append(args, uint64(limit))
	rows, err := r.ch.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]usecaseaudit.DREvent, 0, limit)
	for rows.Next() {
		var e usecaseaudit.DREvent
		var meta string
		if err := rows.Scan(&e.Timestamp, &e.Actor, &e.Action, &e.Target, &e.Status, &e.Message, &meta); err != nil {
			return nil, err
		}
		if meta != "" && meta != "{}" {
			_ = json.Unmarshal([]byte(meta), &e.Meta)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) ListAudit(ctx context.Context, q usecaseaudit.AuditQuery) ([]usecaseaudit.AuditEvent, error) {
	if r == nil || r.ch == nil {
		return nil, fmt.Errorf("clickhouse not configured")
	}
	limit := normalizeListLimit(q.Limit)
	var sb strings.Builder
	args := []any{q.Since}
	sb.WriteString(`SELECT timestamp, actor, action, resource_type, resource_id, result, ip, details FROM audit_events WHERE timestamp >= ?`)
	if v := strings.TrimSpace(q.Action); v != "" {
		sb.WriteString(` AND action = ?`)
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Result); v != "" {
		sb.WriteString(` AND result = ?`)
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Actor); v != "" {
		sb.WriteString(` AND actor = ?`)
		args = append(args, v)
	}
	sb.WriteString(` ORDER BY timestamp DESC LIMIT ?`)
	args = append(args, uint64(limit))
	rows, err := r.ch.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]usecaseaudit.AuditEvent, 0, limit)
	for rows.Next() {
		var e usecaseaudit.AuditEvent
		var details string
		if err := rows.Scan(&e.Timestamp, &e.Actor, &e.Action, &e.ResourceType, &e.ResourceID, &e.Result, &e.IP, &details); err != nil {
			return nil, err
		}
		if details != "" && details != "{}" {
			_ = json.Unmarshal([]byte(details), &e.Details)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
