package bootstrapadapter

import (
	"context"
	"fmt"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"geoatlas/internal/adapter/clickhouse/aggstate"
)

type stubConn struct {
	driver.Conn
}

type stubRow struct{ err error }

func (r stubRow) Err() error           { return r.err }
func (r stubRow) Scan(...any) error    { return r.err }
func (r stubRow) ScanStruct(any) error { return r.err }

func (stubConn) Exec(context.Context, string, ...any) error {
	return fmt.Errorf("stub exec")
}
func (stubConn) Query(context.Context, string, ...any) (driver.Rows, error) {
	return nil, fmt.Errorf("stub query")
}
func (stubConn) QueryRow(context.Context, string, ...any) driver.Row {
	return stubRow{err: fmt.Errorf("stub row")}
}

func TestStorageDelegates(t *testing.T) {
	t.Cleanup(func() {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle", Message: "not started"})
		aggstate.SetGeoEdgesAggReady(false)
		aggstate.SetHourlyEdgesAggReady(false)
	})
	s := &Storage{CH: stubConn{}}
	ctx := context.Background()
	calls := []func() error{
		func() error { return s.EnsureHTTPSchema(ctx) },
		func() error { return s.EnsureTTLOnlyDropParts(ctx) },
		func() error { return s.EnsureTrafficLogsSuccess(ctx) },
		func() error { return s.EnsureEdgesAggSchema(ctx) },
		func() error { return s.EnsureHourlyEdgesAggSchema(ctx) },
		func() error { return s.EnsureReputationRanges(ctx) },
		func() error { return s.BackfillEdgesAgg(ctx) },
		func() error { return s.BackfillGeoEdgesAgg(ctx) },
		func() error { return s.BackfillHourlyEdgesAgg(ctx) },
		func() error { return s.RefreshEdgesAggReady(ctx) },
		func() error { return s.RefreshGeoEdgesAggReady(ctx) },
		func() error { return s.RefreshHourlyEdgesAggReady(ctx) },
	}
	for i, fn := range calls {
		if err := fn(); err == nil {
			t.Fatalf("call %d: expected error from stub CH", i)
		}
	}

	nilCH := &Storage{}
	if err := nilCH.EnsureHTTPSchema(ctx); err == nil {
		t.Fatal("nil CH EnsureHTTPSchema")
	}
	if err := nilCH.EnsureReputationRanges(ctx); err == nil {
		t.Fatal("nil CH EnsureReputationRanges")
	}
}
