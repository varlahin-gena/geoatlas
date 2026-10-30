package perrorstore

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
	execCalls  []string
	execErr    error
	queryQueue []stubQuery
}

type stubQuery struct {
	err  error
	rows [][]any
}

func (c *stubConn) push(rows ...[]any) {
	c.queryQueue = append(c.queryQueue, stubQuery{rows: rows})
}

func (c *stubConn) pushErr(err error) {
	c.queryQueue = append(c.queryQueue, stubQuery{err: err})
}

func (c *stubConn) Exec(_ context.Context, query string, _ ...any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.execCalls = append(c.execCalls, query)
	return c.execErr
}

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
