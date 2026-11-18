package parserstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"geoatlas/internal/parser"
	"geoatlas/internal/usecase/parsers"
)

// Store — ClickHouse persistence for parser_rules.
type Store struct {
	apiCH   clickhouse.Conn
	writeCH clickhouse.Conn
}

func New(apiCH, writeCH clickhouse.Conn) *Store {
	return &Store{apiCH: apiCH, writeCH: writeCH}
}

var _ parsers.Store = (*Store)(nil)

func (s *Store) List(ctx context.Context) ([]parser.RuleRecord, error) {
	if s == nil || s.apiCH == nil {
		return nil, fmt.Errorf("clickhouse conn is nil")
	}
	rows, err := s.apiCH.Query(ctx, `
		SELECT id, name, vendor, enabled, priority, seed, spec_json
		FROM parser_rules FINAL
		WHERE is_deleted = 0
		ORDER BY priority ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list parser_rules: %w", err)
	}
	defer rows.Close()

	out := make([]parser.RuleRecord, 0)
	for rows.Next() {
		var (
			rec      parser.RuleRecord
			enabled  uint8
			seed     uint8
			specJSON string
		)
		if err := rows.Scan(&rec.ID, &rec.Name, &rec.Vendor, &enabled, &rec.Priority, &seed, &specJSON); err != nil {
			return nil, err
		}
		rec.Enabled = enabled != 0
		rec.Seed = seed != 0
		if err := json.Unmarshal([]byte(specJSON), &rec.Spec); err != nil {
			return nil, fmt.Errorf("parse spec for %q: %w", rec.ID, err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) Upsert(ctx context.Context, rec parser.RuleRecord) error {
	if s == nil || s.writeCH == nil {
		return fmt.Errorf("clickhouse conn is nil")
	}
	specJSON, err := json.Marshal(rec.Spec)
	if err != nil {
		return err
	}
	var en, seed uint8
	if rec.Enabled {
		en = 1
	}
	if rec.Seed {
		seed = 1
	}
	now := time.Now().UTC()
	ver := uint64(now.UnixNano())
	return s.writeCH.Exec(ctx, `
		INSERT INTO parser_rules
		(id, name, vendor, enabled, priority, seed, spec_json, updated_at, ver, is_deleted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
	`, rec.ID, rec.Name, rec.Vendor, en, int32(rec.Priority), seed, string(specJSON), now, ver)
}

func (s *Store) Delete(ctx context.Context, id string) error {
	if s == nil || s.writeCH == nil {
		return fmt.Errorf("clickhouse conn is nil")
	}
	now := time.Now().UTC()
	ver := uint64(now.UnixNano())
	return s.writeCH.Exec(ctx, `
		INSERT INTO parser_rules
		(id, name, vendor, enabled, priority, seed, spec_json, updated_at, ver, is_deleted)
		VALUES (?, '', '', 0, 0, 0, '', ?, ?, 1)
	`, id, now, ver)
}

func (s *Store) Exists(ctx context.Context, id string) (bool, error) {
	if s == nil || s.apiCH == nil {
		return false, fmt.Errorf("clickhouse conn is nil")
	}
	var n uint64
	err := s.apiCH.QueryRow(ctx, `
		SELECT count() FROM parser_rules FINAL WHERE id = ? AND is_deleted = 0
	`, id).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
