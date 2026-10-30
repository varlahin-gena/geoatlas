package sysstore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type stubConn struct {
	driver.Conn
	mu         sync.Mutex
	queryQueue []stubQuery
	rowQueue   []stubRow
	pingErr    error
}

type stubQuery struct {
	err  error
	rows [][]any
}

type stubRow struct {
	err  error
	vals []any
}

func (c *stubConn) pushQuery(rows ...[]any) {
	c.queryQueue = append(c.queryQueue, stubQuery{rows: rows})
}

func (c *stubConn) pushQueryErr(err error) {
	c.queryQueue = append(c.queryQueue, stubQuery{err: err})
}

func (c *stubConn) pushRow(vals ...any) {
	c.rowQueue = append(c.rowQueue, stubRow{vals: vals})
}

func (c *stubConn) pushRowErr(err error) {
	c.rowQueue = append(c.rowQueue, stubRow{err: err})
}

func (c *stubConn) Ping(context.Context) error { return c.pingErr }

func (c *stubConn) Query(_ context.Context, _ string, _ ...any) (driver.Rows, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.queryQueue) == 0 {
		return nil, fmt.Errorf("stubConn: unexpected Query")
	}
	q := c.queryQueue[0]
	c.queryQueue = c.queryQueue[1:]
	if q.err != nil {
		return nil, q.err
	}
	return &stubRows{rows: q.rows}, nil
}

func (c *stubConn) QueryRow(_ context.Context, _ string, _ ...any) driver.Row {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.rowQueue) == 0 {
		return stubRow{err: fmt.Errorf("stubConn: unexpected QueryRow")}
	}
	r := c.rowQueue[0]
	c.rowQueue = c.rowQueue[1:]
	return r
}

func (r stubRow) Err() error { return r.err }

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.vals) {
		return fmt.Errorf("stubRow.Scan dest=%d vals=%d", len(dest), len(r.vals))
	}
	for i := range dest {
		if err := assignAny(dest[i], r.vals[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r stubRow) ScanStruct(any) error { return fmt.Errorf("ScanStruct not implemented") }

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
		return fmt.Errorf("Scan before Next")
	}
	vals := r.rows[r.i-1]
	if len(dest) != len(vals) {
		return fmt.Errorf("stubRows.Scan mismatch")
	}
	for i := range dest {
		if err := assignAny(dest[i], vals[i]); err != nil {
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

func assignAny(dest, val any) error {
	switch d := dest.(type) {
	case *string:
		*d = fmt.Sprint(val)
	case *float64:
		switch v := val.(type) {
		case float64:
			*d = v
		case int:
			*d = float64(v)
		default:
			return fmt.Errorf("float64 from %T", val)
		}
	case *uint64:
		switch v := val.(type) {
		case uint64:
			*d = v
		case int:
			*d = uint64(v)
		default:
			return fmt.Errorf("uint64 from %T", val)
		}
	case *time.Time:
		t, ok := val.(time.Time)
		if !ok {
			return fmt.Errorf("time from %T", val)
		}
		*d = t
	default:
		return fmt.Errorf("unsupported dest %T", dest)
	}
	return nil
}

func metricLatestVals(ts time.Time, value float64) []any {
	return []any{ts, "cpu", "host", "usage", value, "{}"}
}

func metricHistVals(ts time.Time, value float64) []any {
	return []any{ts, "cpu", "host", "usage", value}
}
