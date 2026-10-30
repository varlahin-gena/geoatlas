package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"geoatlas/internal/model"
)

type stubConn struct {
	driver.Conn
	mu        sync.Mutex
	execCalls []string
	execErr   error
	rowQueue  []stubRow
	closed    bool
	closeErr  error
}

type stubRow struct {
	err  error
	vals []any
}

func (c *stubConn) pushRow(vals ...any) {
	c.rowQueue = append(c.rowQueue, stubRow{vals: vals})
}

func (c *stubConn) Exec(_ context.Context, query string, _ ...any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.execCalls = append(c.execCalls, query)
	return c.execErr
}

func (c *stubConn) QueryRow(_ context.Context, _ string, _ ...any) driver.Row {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.rowQueue) == 0 {
		return stubRow{err: fmt.Errorf("unexpected QueryRow")}
	}
	r := c.rowQueue[0]
	c.rowQueue = c.rowQueue[1:]
	return r
}

func (c *stubConn) Query(context.Context, string, ...any) (driver.Rows, error) {
	return nil, fmt.Errorf("unexpected Query")
}

func (c *stubConn) Close() error {
	c.closed = true
	return c.closeErr
}

func (r stubRow) Err() error { return r.err }
func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.vals) {
		return fmt.Errorf("scan mismatch")
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
			return fmt.Errorf("unsupported %T", dest[i])
		}
	}
	return nil
}
func (r stubRow) ScanStruct(any) error { return fmt.Errorf("n/a") }

type stubGeo struct{}

func (stubGeo) RangeCount() int               { return 1 }
func (stubGeo) Lookup(string) model.GeoLookup { return model.GeoLookup{} }

func TestMaintenanceStoreNilSafe(t *testing.T) {
	var m *MaintenanceStore
	ctx := context.Background()
	if err := m.BackfillEdgesAgg(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.BackfillGeoEdgesAgg(ctx); err != nil {
		t.Fatal(err)
	}
	if n, err := m.EnrichLogsMissingGeo(ctx, stubGeo{}, 1); err != nil || n != 0 {
		t.Fatalf("%d %v", n, err)
	}
	if err := m.RebuildGeoEdgesLookback(ctx, 1); err != nil {
		t.Fatal(err)
	}

	m = NewMaintenanceStore(nil)
	if err := m.BackfillEdgesAgg(ctx); err != nil {
		t.Fatal(err)
	}
	if n, err := m.EnrichLogsMissingGeo(ctx, nil, 1); err != nil || n != 0 {
		t.Fatal(n, err)
	}
}

func TestMaintenanceStoreDelegates(t *testing.T) {
	ch := &stubConn{rowQueue: []stubRow{{err: fmt.Errorf("count fail")}}}
	if err := NewMaintenanceStore(ch).BackfillEdgesAgg(context.Background()); err == nil {
		t.Fatal("want backfill err")
	}
	ch2 := &stubConn{rowQueue: []stubRow{{err: fmt.Errorf("geo fail")}}}
	if err := NewMaintenanceStore(ch2).BackfillGeoEdgesAgg(context.Background()); err == nil {
		t.Fatal("want geo err")
	}
	ch3 := &stubConn{rowQueue: []stubRow{{err: fmt.Errorf("enrich fail")}}}
	if _, err := NewMaintenanceStore(ch3).EnrichLogsMissingGeo(context.Background(), stubGeo{}, 1); err == nil {
		t.Fatal("want enrich err")
	}
	ch4 := &stubConn{rowQueue: []stubRow{{err: fmt.Errorf("rebuild fail")}}}
	if err := NewMaintenanceStore(ch4).RebuildGeoEdgesLookback(context.Background(), 1); err == nil {
		t.Fatal("want rebuild err")
	}
}

func TestRetentionApplierPaths(t *testing.T) {
	var nilA *RetentionApplier
	if err := nilA.ApplyTrafficLogs(context.Background(), 30); err == nil {
		t.Fatal("nil applier")
	}
	a := NewRetentionApplier(nil)
	if err := a.ApplyParseErrors(context.Background(), 7); err == nil {
		t.Fatal("nil conn")
	}

	ch := &stubConn{}
	a = NewRetentionApplier(ch)
	for i := 0; i < 10; i++ {
		ch.pushRow(uint64(0))
	}
	if err := a.ApplyTrafficLogs(context.Background(), 30); err != nil {
		t.Fatal(err)
	}
	if err := a.ApplyParseErrors(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if err := a.ApplySystemMetrics(context.Background(), 14); err != nil {
		t.Fatal(err)
	}
	if err := a.ApplyEdges(context.Background(), 30); err != nil {
		t.Fatal(err)
	}

	ch2 := &stubConn{}
	ch2.pushRow(uint64(1))
	ch2.pushRow(uint64(1))
	if err := NewRetentionApplier(ch2).ApplyTrafficLogs(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if len(ch2.execCalls) != 1 {
		t.Fatalf("exec=%v", ch2.execCalls)
	}

	ch3 := &stubConn{execErr: fmt.Errorf("alter fail")}
	ch3.pushRow(uint64(1))
	ch3.pushRow(uint64(1))
	if err := NewRetentionApplier(ch3).ApplySystemMetrics(context.Background(), 3); err == nil {
		t.Fatal("want alter err")
	}

	ch4 := &stubConn{}
	ch4.pushRow(uint64(0)) // daily missing
	ch4.pushRow(uint64(1)) // hourly exists
	ch4.pushRow(uint64(1)) // hourly modifyTTL exists
	ch4.pushRow(uint64(0))
	ch4.pushRow(uint64(0))
	ch4.pushRow(uint64(0))
	if err := NewRetentionApplier(ch4).ApplyEdges(context.Background(), 30); err != nil {
		t.Fatal(err)
	}
	if len(ch4.execCalls) != 1 || !strings.Contains(ch4.execCalls[0], "INTERVAL 7 DAY") {
		t.Fatalf("hourly clamp: %v", ch4.execCalls)
	}

	ch5 := &stubConn{rowQueue: []stubRow{{err: fmt.Errorf("exists fail")}}}
	if err := NewRetentionApplier(ch5).ApplyEdges(context.Background(), 5); err == nil {
		t.Fatal("want exists err")
	}
}

func TestPoolsClose(t *testing.T) {
	var p *Pools
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	c1, c2, c3 := &stubConn{}, &stubConn{closeErr: fmt.Errorf("c2")}, &stubConn{}
	p = &Pools{Ingest: c1, API: c2, Background: c3}
	if err := p.Close(); err == nil {
		t.Fatal("want first close err")
	}
	if !c1.closed || !c2.closed || !c3.closed {
		t.Fatal("not all closed")
	}
	if err := (&Pools{}).Close(); err != nil {
		t.Fatal(err)
	}
}
