package query

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
		return fmt.Errorf("stubRows.Scan dest=%d vals=%d", len(dest), len(vals))
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
	case *uint64:
		switch v := val.(type) {
		case uint64:
			*d = v
		case int:
			*d = uint64(v)
		default:
			return fmt.Errorf("uint64 from %T", val)
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
			return fmt.Errorf("uint32 from %T", val)
		}
	case *float64:
		switch v := val.(type) {
		case float64:
			*d = v
		case int:
			*d = float64(v)
		default:
			return fmt.Errorf("float64 from %T", val)
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

func rawAggVals() []any {
	return []any{
		"1.1.1.1", "2.2.2.2", uint64(3), uint64(1), uint64(2),
		"allow", "r1", "tcp", uint32(10), uint32(443),
		"fw", "lan", "wan", "RU", "US", "Moscow", "NYC",
		55.0, 37.0, 40.0, -74.0,
		uint64(100), uint64(200), uint64(5), uint64(6),
	}
}

func geoEdgeVals() []any {
	return []any{
		"src", "dst", "Src", "Dst",
		uint64(10), uint64(2), uint64(8),
		uint64(100), uint64(50), uint64(3), uint64(4),
		110.0, 74.0, 80.0, -148.0, uint64(2),
		"allow", "r", "tcp", uint32(1), uint32(2),
		"fw", "z1", "z2", "RU", "US", "A", "B",
	}
}
