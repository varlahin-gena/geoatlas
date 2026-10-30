package anomalystore

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// stubConn — sequential Query/QueryRow/Exec/PrepareBatch fake for Repository tests.
type stubConn struct {
	driver.Conn
	mu sync.Mutex

	execCalls []string
	execErr   error

	queryQueue    []stubQuery
	queryRowQueue []stubRow

	batchErr    error
	appendErr   error
	sendErr     error
	prepareErr  error
	batchAppend int
}

type stubQuery struct {
	err  error
	rows [][]any
}

func (c *stubConn) pushQuery(rows ...[]any) {
	c.queryQueue = append(c.queryQueue, stubQuery{rows: rows})
}

func (c *stubConn) pushQueryErr(err error) {
	c.queryQueue = append(c.queryQueue, stubQuery{err: err})
}

func (c *stubConn) pushRow(vals ...any) {
	c.queryRowQueue = append(c.queryRowQueue, stubRow{vals: vals})
}

func (c *stubConn) pushRowErr(err error) {
	c.queryRowQueue = append(c.queryRowQueue, stubRow{err: err})
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

func (c *stubConn) QueryRow(_ context.Context, _ string, _ ...any) driver.Row {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.queryRowQueue) == 0 {
		return stubRow{err: fmt.Errorf("stubConn: unexpected QueryRow")}
	}
	r := c.queryRowQueue[0]
	c.queryRowQueue = c.queryRowQueue[1:]
	return r
}

func (c *stubConn) PrepareBatch(context.Context, string, ...driver.PrepareBatchOption) (driver.Batch, error) {
	if c.prepareErr != nil {
		return nil, c.prepareErr
	}
	return &stubBatch{conn: c}, nil
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
		return fmt.Errorf("stubRow.Scan: dest=%d vals=%d", len(dest), len(r.vals))
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

type stubBatch struct {
	conn *stubConn
	sent bool
}

func (b *stubBatch) Abort() error { return nil }
func (b *stubBatch) Append(v ...any) error {
	if b.conn.appendErr != nil {
		return b.conn.appendErr
	}
	b.conn.batchAppend++
	return nil
}
func (b *stubBatch) AppendStruct(any) error { return nil }
func (b *stubBatch) Column(int) driver.BatchColumn {
	return stubBatchCol{}
}
func (b *stubBatch) Flush() error { return nil }
func (b *stubBatch) Send() error {
	b.sent = true
	return b.conn.sendErr
}
func (b *stubBatch) IsSent() bool                { return b.sent }
func (b *stubBatch) Rows() int                   { return b.conn.batchAppend }
func (b *stubBatch) Columns() []column.Interface { return nil }
func (b *stubBatch) Close() error                { return nil }

type stubBatchCol struct{}

func (stubBatchCol) Append(any) error    { return nil }
func (stubBatchCol) AppendRow(any) error { return nil }

func assignAny(dest, val any) error {
	switch d := dest.(type) {
	case *string:
		*d = fmt.Sprint(val)
	case *uint64:
		switch v := val.(type) {
		case uint64:
			*d = v
		case int:
			*d = uint64(v)
		case int64:
			*d = uint64(v)
		default:
			return fmt.Errorf("uint64 from %T", val)
		}
	case *uint8:
		switch v := val.(type) {
		case uint8:
			*d = v
		case int:
			*d = uint8(v)
		default:
			return fmt.Errorf("uint8 from %T", val)
		}
	case *float32:
		switch v := val.(type) {
		case float32:
			*d = v
		case float64:
			*d = float32(v)
		case int:
			*d = float32(v)
		default:
			return fmt.Errorf("float32 from %T", val)
		}
	case *time.Time:
		t, ok := val.(time.Time)
		if !ok {
			return fmt.Errorf("time from %T", val)
		}
		*d = t
	case *[]int64:
		switch v := val.(type) {
		case []int64:
			*d = v
		case nil:
			*d = nil
		default:
			return fmt.Errorf("[]int64 from %T", val)
		}
	default:
		return fmt.Errorf("unsupported dest %T", dest)
	}
	return nil
}
