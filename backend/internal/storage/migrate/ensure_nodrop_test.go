package migrate

import (
	"context"
	"errors"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/storage/aggstate"
)

func TestEnsureEdgesAggNoDropWhenSchemaVersionErrors(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() {
		needsSchemaDDLFn = old
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle", Message: "not started"})
	})

	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		// needDDL=true «обманул бы» старый код в DROP; при err Ensure обязан выйти раньше.
		return true, errors.New("clickhouse unavailable")
	}

	// ch=nil безопасен: Ensure возвращает до applyEdgesAggSchema / Exec(DROP).
	err := EnsureEdgesAgg(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error from needsSchemaDDL")
	}
	if aggstate.GetEdgesAggStatus().State != "error" {
		t.Fatalf("status=%q want error", aggstate.GetEdgesAggStatus().State)
	}
	if aggstate.PreferDailyEdgesAgg() {
		t.Fatal("edges agg must not be ready after schema error")
	}
}

func TestEnsureGeoEdgesAggNoDropWhenSchemaVersionErrors(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() {
		needsSchemaDDLFn = old
		aggstate.SetGeoEdgesAggReady(false)
	})

	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return true, errors.New("clickhouse blip")
	}

	aggstate.SetGeoEdgesAggReady(true)
	err := EnsureGeoEdgesAgg(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if aggstate.PreferGeoEdgesAgg() {
		t.Fatal("geo edges must not stay ready after ensure failure")
	}
}

func TestEnsureEdgesAggSchemaSetsPending(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() {
		needsSchemaDDLFn = old
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle", Message: "not started"})
	})

	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return false, nil // schema already at version — no DDL
	}

	if err := EnsureEdgesAggSchema(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	st := aggstate.GetEdgesAggStatus()
	if st.State != "pending" {
		t.Fatalf("state=%q want pending", st.State)
	}
	if aggstate.PreferDailyEdgesAgg() {
		t.Fatal("prefer must stay false until backfill")
	}
}

func TestEnsureTrafficLogsSuccessNoDDLWhenSchemaVersionErrors(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() { needsSchemaDDLFn = old })

	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return true, errors.New("ch down")
	}
	if err := EnsureTrafficLogsSuccess(context.Background(), nil); err == nil {
		t.Fatal("expected error")
	}
}

