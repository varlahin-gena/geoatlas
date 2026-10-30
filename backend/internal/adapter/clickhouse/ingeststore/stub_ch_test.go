package ingeststore

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type stubConn struct {
	driver.Conn
	prepareErr  error
	appendErr   error
	colAppendEr error
	sendErr     error
	batchAppend int
	colAppends  int
}

func (c *stubConn) PrepareBatch(context.Context, string, ...driver.PrepareBatchOption) (driver.Batch, error) {
	if c.prepareErr != nil {
		return nil, c.prepareErr
	}
	return &stubBatch{conn: c}, nil
}

type stubBatch struct {
	conn *stubConn
	sent bool
}

func (b *stubBatch) Abort() error { return nil }
func (b *stubBatch) Append(...any) error {
	if b.conn.appendErr != nil {
		return b.conn.appendErr
	}
	b.conn.batchAppend++
	return nil
}
func (b *stubBatch) AppendStruct(any) error { return nil }
func (b *stubBatch) Column(int) driver.BatchColumn {
	return stubBatchCol{conn: b.conn}
}
func (b *stubBatch) Flush() error                { return nil }
func (b *stubBatch) Send() error                 { b.sent = true; return b.conn.sendErr }
func (b *stubBatch) IsSent() bool                { return b.sent }
func (b *stubBatch) Rows() int                   { return b.conn.batchAppend }
func (b *stubBatch) Columns() []column.Interface { return nil }
func (b *stubBatch) Close() error                { return nil }

type stubBatchCol struct{ conn *stubConn }

func (c stubBatchCol) Append(any) error {
	if c.conn.colAppendEr != nil {
		return c.conn.colAppendEr
	}
	c.conn.colAppends++
	return nil
}
func (stubBatchCol) AppendRow(any) error { return nil }
