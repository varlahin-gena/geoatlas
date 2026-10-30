package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// stubConn — минимальный clickhouse.Conn для unit-тестов Ensure*/backfill.
// Embed driver.Conn: новые методы драйвера не ломают сборку.
type stubConn struct {
	driver.Conn

	mu sync.Mutex

	execCalls []string
	execErr   error
	failSub   map[string]error // substring → error on Exec

	// tables that exist (system.tables count)
	tables map[string]bool
	// row counts from system.parts
	rows map[string]uint64
	// "table.column" → type
	colType map[string]string
	// "table.column" → exists (default: present if colType set, else false unless listed)
	colOK map[string]bool
	// table → partition_key / sorting_key
	partKey map[string]string
	sortKey map[string]string
	// table → partition_id for any day (empty = no partition)
	partID map[string]string

	schemaVer map[string]uint32

	// rows returned by missingClosedPartitionDays / listLookbackClosedDays
	missingDays  []time.Time
	lookbackDays []time.Time

	queryErr error // global Query/QueryRow failure
}

func newStubConn() *stubConn {
	return &stubConn{
		tables:    map[string]bool{},
		rows:      map[string]uint64{},
		colType:   map[string]string{},
		colOK:     map[string]bool{},
		partKey:   map[string]string{},
		sortKey:   map[string]string{},
		partID:    map[string]string{},
		schemaVer: map[string]uint32{},
		failSub:   map[string]error{},
	}
}

func (c *stubConn) Exec(_ context.Context, query string, _ ...any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.execCalls = append(c.execCalls, query)
	if c.execErr != nil {
		return c.execErr
	}
	for sub, err := range c.failSub {
		if strings.Contains(query, sub) {
			return err
		}
	}
	return nil
}

func (c *stubConn) QueryRow(_ context.Context, query string, args ...any) driver.Row {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.queryErr != nil {
		return stubRow{err: c.queryErr}
	}
	q := strings.ToLower(query)

	switch {
	case strings.Contains(q, "from ga_schema_version"):
		comp := argString(args, 0)
		v, ok := c.schemaVer[comp]
		if !ok {
			return stubRow{err: sql.ErrNoRows}
		}
		return stubRow{vals: []any{v}}

	case strings.Contains(q, "from system.tables") && strings.Contains(q, "partition_key"):
		table := namedOrPos(args, "table", 0)
		pk, ok := c.partKey[table]
		if !ok {
			return stubRow{err: sql.ErrNoRows}
		}
		return stubRow{vals: []any{pk}}

	case strings.Contains(q, "from system.tables") && strings.Contains(q, "sorting_key"):
		table := namedOrPos(args, "table", 0)
		sk, ok := c.sortKey[table]
		if !ok {
			return stubRow{err: fmt.Errorf("sorting_key missing")}
		}
		return stubRow{vals: []any{sk}}

	case strings.Contains(q, "from system.tables") && strings.Contains(q, "count()"):
		name := namedOrPos(args, "name", 0)
		var n uint64
		if c.tables[name] {
			n = 1
		}
		return stubRow{vals: []any{n}}

	case strings.Contains(q, "from system.columns") && strings.Contains(q, "select type"):
		table := namedOrPos(args, "table", 0)
		col := namedOrPos(args, "column", 1)
		key := table + "." + col
		typ, ok := c.colType[key]
		if !ok {
			return stubRow{err: fmt.Errorf("column type %s: no rows", key)}
		}
		return stubRow{vals: []any{typ}}

	case strings.Contains(q, "from system.columns") && strings.Contains(q, "count()"):
		table := namedOrPos(args, "table", 0)
		col := namedOrPos(args, "column", 1)
		key := table + "." + col
		var n uint64
		if c.colOK[key] || c.colType[key] != "" {
			n = 1
		}
		return stubRow{vals: []any{n}}

	case strings.Contains(q, "from system.parts") && strings.Contains(q, "sum(rows)"):
		table := namedOrPos(args, "table", 0)
		return stubRow{vals: []any{c.rows[table]}}

	case strings.Contains(q, "from system.parts") && strings.Contains(q, "partition_id"):
		table := namedOrPos(args, "table", 0)
		id := c.partID[table]
		if id == "" {
			return stubRow{err: sql.ErrNoRows}
		}
		return stubRow{vals: []any{id}}
	}

	return stubRow{err: fmt.Errorf("stubConn: unhandled QueryRow: %s", trimQ(query))}
}

func (c *stubConn) Query(_ context.Context, query string, _ ...any) (driver.Rows, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	q := strings.ToLower(query)
	days := c.missingDays
	if strings.Contains(q, "table = 'traffic_logs'") || (!strings.Contains(q, "not in") && strings.Contains(q, "from system.parts")) {
		// listLookbackClosedDays has literal 'traffic_logs' and no NOT IN.
		if !strings.Contains(q, "not in") {
			days = c.lookbackDays
			if days == nil {
				days = c.missingDays
			}
		}
	}
	vals := make([][]any, 0, len(days))
	for _, d := range days {
		vals = append(vals, []any{d})
	}
	return &stubRows{rows: vals}, nil
}

func namedOrPos(args []any, name string, pos int) string {
	for _, a := range args {
		switch v := a.(type) {
		case driver.NamedValue:
			if v.Name == name {
				return fmt.Sprint(v.Value)
			}
		}
	}
	if pos >= 0 && pos < len(args) {
		switch v := args[pos].(type) {
		case driver.NamedValue:
			return fmt.Sprint(v.Value)
		case string:
			return v
		default:
			return fmt.Sprint(v)
		}
	}
	return ""
}

func argString(args []any, pos int) string {
	if pos < 0 || pos >= len(args) {
		return ""
	}
	switch v := args[pos].(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func trimQ(q string) string {
	q = strings.TrimSpace(q)
	if len(q) > 120 {
		return q[:120] + "…"
	}
	return q
}

type stubRow struct {
	err  error
	vals []any
}

func (r stubRow) Err() error { return r.err }

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.vals) {
		return fmt.Errorf("stubRow.Scan: got %d dest, have %d vals", len(dest), len(r.vals))
	}
	for i := range dest {
		if err := assign(dest[i], r.vals[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r stubRow) ScanStruct(any) error { return fmt.Errorf("stubRow: ScanStruct not implemented") }

type stubRows struct {
	rows [][]any
	i    int
	err  error
}

func (r *stubRows) Next() bool {
	if r.i >= len(r.rows) {
		return false
	}
	r.i++
	return true
}

func (r *stubRows) Scan(dest ...any) error {
	if r.i == 0 || r.i > len(r.rows) {
		return fmt.Errorf("stubRows: Scan before Next")
	}
	vals := r.rows[r.i-1]
	if len(dest) != len(vals) {
		return fmt.Errorf("stubRows.Scan: dest/vals mismatch")
	}
	for i := range dest {
		if err := assign(dest[i], vals[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *stubRows) ScanStruct(any) error             { return fmt.Errorf("not implemented") }
func (r *stubRows) ColumnTypes() []driver.ColumnType { return nil }
func (r *stubRows) Totals(...any) error              { return nil }
func (r *stubRows) Columns() []string                { return nil }
func (r *stubRows) Close() error                     { return nil }
func (r *stubRows) Err() error                       { return r.err }
func (r *stubRows) HasData() bool                    { return len(r.rows) > 0 }

func assign(dest, val any) error {
	switch d := dest.(type) {
	case *uint64:
		switch v := val.(type) {
		case uint64:
			*d = v
		case int:
			*d = uint64(v)
		default:
			return fmt.Errorf("assign uint64 from %T", val)
		}
	case *uint32:
		switch v := val.(type) {
		case uint32:
			*d = v
		case int:
			*d = uint32(v)
		case uint64:
			*d = uint32(v)
		default:
			return fmt.Errorf("assign uint32 from %T", val)
		}
	case *string:
		*d = fmt.Sprint(val)
	case *time.Time:
		t, ok := val.(time.Time)
		if !ok {
			return fmt.Errorf("assign time.Time from %T", val)
		}
		*d = t
	case *bool:
		b, ok := val.(bool)
		if !ok {
			return fmt.Errorf("assign bool from %T", val)
		}
		*d = b
	default:
		return fmt.Errorf("assign: unsupported dest %T", dest)
	}
	return nil
}
