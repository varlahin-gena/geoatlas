package migrate

import (
	"context"
	"errors"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/clickhouse/aggstate"
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

func TestEnsureHourlyEdgesAggNoDropWhenSchemaVersionErrors(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() {
		needsSchemaDDLFn = old
		aggstate.SetHourlyEdgesAggReady(false)
	})

	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return true, errors.New("clickhouse blip")
	}
	aggstate.SetHourlyEdgesAggReady(true)
	if err := EnsureHourlyEdgesAgg(context.Background(), nil); err == nil {
		t.Fatal("expected error")
	}
	if aggstate.PreferHourlyEdgesAgg() {
		t.Fatal("hourly must not stay ready after ensure failure")
	}
}

func TestEnsureTrafficLogsIPv4NoDDLWhenSchemaVersionErrors(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() { needsSchemaDDLFn = old })

	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return true, errors.New("ch down")
	}
	if err := EnsureTrafficLogsIPv4(context.Background(), nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestIsIPv4Type(t *testing.T) {
	if !isIPv4Type("IPv4") {
		t.Fatal("IPv4")
	}
	if isIPv4Type("String") || isIPv4Type("IPv6") {
		t.Fatal("non-IPv4")
	}
}

func TestIsDayPartitionKey(t *testing.T) {
	if !isDayPartitionKey("day") || !isDayPartitionKey(" day ") || !isDayPartitionKey("`day`") {
		t.Fatal("want day partition accepted")
	}
	if isDayPartitionKey("toYYYYMM(day)") || isDayPartitionKey("toYYYYMMDD(day)") {
		t.Fatal("monthly/daily-func partition must not count as day")
	}
}
