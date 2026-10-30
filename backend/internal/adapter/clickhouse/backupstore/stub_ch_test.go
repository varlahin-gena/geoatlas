package backupstore

import (
	"context"
	"fmt"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type stubConn struct {
	driver.Conn
	mu        sync.Mutex
	execCalls []string
	execErrs  []error
	rowQueue  []stubRow
}

type stubRow struct {
	err  error
	vals []any
}

func (c *stubConn) pushRow(vals ...any) {
	c.rowQueue = append(c.rowQueue, stubRow{vals: vals})
}

func (c *stubConn) pushRowErr(err error) {
	c.rowQueue = append(c.rowQueue, stubRow{err: err})
}

func (c *stubConn) queueExecErr(err error) {
	c.execErrs = append(c.execErrs, err)
}

func (c *stubConn) Exec(_ context.Context, query string, _ ...any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.execCalls = append(c.execCalls, query)
	if len(c.execErrs) > 0 {
		err := c.execErrs[0]
		c.execErrs = c.execErrs[1:]
		return err
	}
	return nil
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
		switch d := dest[i].(type) {
		case *uint64:
			switch v := r.vals[i].(type) {
			case uint64:
				*d = v
			case int:
				*d = uint64(v)
			default:
				return fmt.Errorf("uint64 from %T", r.vals[i])
			}
		default:
			return fmt.Errorf("unsupported dest %T", dest[i])
		}
	}
	return nil
}

func (r stubRow) ScanStruct(any) error { return fmt.Errorf("ScanStruct not implemented") }
