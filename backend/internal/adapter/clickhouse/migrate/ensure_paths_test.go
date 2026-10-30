package migrate

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"geoatlas/internal/adapter/clickhouse/aggstate"
	"geoatlas/internal/adapter/clickhouse/sqlclause"
)

func TestEnsureBaseSchemaNilAndOK(t *testing.T) {
	if err := EnsureBaseSchema(context.Background(), nil); err == nil {
		t.Fatal("nil conn")
	}
	ch := newStubConn()
	if err := EnsureBaseSchema(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
	if len(ch.execCalls) < 10 {
		t.Fatalf("expected cold bootstrap DDLs, got %d", len(ch.execCalls))
	}
}

func TestEnsureReputationRanges(t *testing.T) {
	if err := EnsureReputationRanges(context.Background(), nil); err == nil {
		t.Fatal("nil")
	}
	ch := newStubConn()
	if err := EnsureReputationRanges(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
	ch.execErr = errors.New("boom")
	if err := EnsureReputationRanges(context.Background(), ch); err == nil {
		t.Fatal("want exec error")
	}
}

func TestEnsureTrafficLogsSuccessAppliesDDL(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() { needsSchemaDDLFn = old })
	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return true, nil
	}
	ch := newStubConn()
	if err := EnsureTrafficLogsSuccess(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(ch.execCalls, "\n")
	if !strings.Contains(joined, "MODIFY COLUMN success") {
		t.Fatalf("missing success DDL: %s", joined)
	}
	if !strings.Contains(joined, "INSERT INTO ga_schema_version") {
		t.Fatal("missing setSchemaVersion")
	}
}

func TestEnsureTrafficLogsSuccessSkipWhenCurrent(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() { needsSchemaDDLFn = old })
	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return false, nil
	}
	ch := newStubConn()
	if err := EnsureTrafficLogsSuccess(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
	if len(ch.execCalls) != 0 {
		t.Fatalf("unexpected exec: %#v", ch.execCalls)
	}
}

func TestEnsureTTLOnlyDropParts(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() { needsSchemaDDLFn = old })
	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return true, nil
	}
	ch := newStubConn()
	ch.tables["traffic_logs"] = true
	ch.tables["anomaly_events"] = true
	// other tables missing → skip
	if err := EnsureTTLOnlyDropParts(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, q := range ch.execCalls {
		if strings.Contains(q, "ttl_only_drop_parts") {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("ttl alters=%d want 2; calls=%#v", n, ch.execCalls)
	}
}

func TestEnsureTTLOnlyDropPartsSkip(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() { needsSchemaDDLFn = old })
	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return false, nil
	}
	if err := EnsureTTLOnlyDropParts(context.Background(), newStubConn()); err != nil {
		t.Fatal(err)
	}
}

func TestNeedsSchemaDDLWithStub(t *testing.T) {
	ch := newStubConn()
	// first call creates version table + no row → need DDL
	need, err := needsSchemaDDL(context.Background(), ch, "comp_x", 3)
	if err != nil {
		t.Fatal(err)
	}
	if !need {
		t.Fatal("want need DDL when version missing")
	}
	ch.schemaVer["comp_x"] = 3
	need, err = needsSchemaDDL(context.Background(), ch, "comp_x", 3)
	if err != nil || need {
		t.Fatalf("need=%v err=%v want false", need, err)
	}
}

func TestSchemaVersionHelpers(t *testing.T) {
	ch := newStubConn()
	if err := ensureSchemaVersionTable(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
	v, err := schemaVersion(context.Background(), ch, "missing")
	if err != nil || v != 0 {
		t.Fatalf("v=%d err=%v", v, err)
	}
	if err := setSchemaVersion(context.Background(), ch, "c1", 2); err != nil {
		t.Fatal(err)
	}
	ch.schemaVer["c1"] = 2
	v, err = schemaVersion(context.Background(), ch, "c1")
	if err != nil || v != 2 {
		t.Fatalf("v=%d err=%v", v, err)
	}
}

func TestPartsHelpers(t *testing.T) {
	ctx := context.Background()
	ch := newStubConn()

	if _, err := countTableRows(ctx, ch, "bad-name!"); err == nil {
		t.Fatal("invalid table")
	}
	ch.rows["traffic_logs"] = 42
	n, err := countTableRows(ctx, ch, "traffic_logs")
	if err != nil || n != 42 {
		t.Fatalf("n=%d err=%v", n, err)
	}

	if partitionDayExpr() == "" {
		t.Fatal("partitionDayExpr empty")
	}

	ch.missingDays = []time.Time{
		time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 8, 2, 0, 0, 0, 0, time.UTC),
	}
	days, err := missingClosedPartitionDays(ctx, ch, "traffic_logs", "traffic_edges_daily")
	if err != nil || len(days) != 2 {
		t.Fatalf("days=%v err=%v", days, err)
	}
	days, err = missingClosedPartitionDaysSince(ctx, ch, "traffic_logs", "traffic_edges_daily", 7)
	if err != nil || len(days) != 2 {
		t.Fatalf("since days=%v err=%v", days, err)
	}
	if _, err := missingClosedPartitionDays(ctx, ch, "bad!", "x"); err == nil {
		t.Fatal("invalid names")
	}

	if err := dropDatePartition(ctx, ch, "bad!", time.Now()); err == nil {
		t.Fatal("invalid drop")
	}
	// no partition id → noop
	if err := dropDatePartition(ctx, ch, "traffic_edges_daily", time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	ch.partID["traffic_edges_daily"] = "20240801"
	if err := dropDatePartition(ctx, ch, "traffic_edges_daily", time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	ch.partID["traffic_edges_daily"] = "bad'id"
	if err := dropDatePartition(ctx, ch, "traffic_edges_daily", time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("unsafe id")
	}

	ch.colOK["t.c"] = true
	ok, err := columnExists(ctx, ch, "t", "c")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	ch.sortKey["t"] = "a, b"
	sk, err := tableSortingKey(ctx, ch, "t")
	if err != nil || sk != "a, b" {
		t.Fatalf("sk=%q err=%v", sk, err)
	}
}

func TestScanDays(t *testing.T) {
	rows := &stubRows{rows: [][]any{
		{time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)},
	}}
	days, err := scanDays(rows)
	if err != nil || len(days) != 1 {
		t.Fatalf("days=%v err=%v", days, err)
	}
}

func TestExecDDL(t *testing.T) {
	ch := newStubConn()
	if err := execDDL(context.Background(), ch, "SELECT 1"); err != nil {
		t.Fatal(err)
	}
}

func TestTableExistsAndReplaceMV(t *testing.T) {
	ctx := context.Background()
	ch := newStubConn()
	ok, err := tableExists(ctx, ch, "mv_x")
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	// create path
	if err := replaceMaterializedView(ctx, ch, "mv_x", func(name string) string {
		return "CREATE MATERIALIZED VIEW " + name + " AS SELECT 1"
	}); err != nil {
		t.Fatal(err)
	}
	ch.tables["mv_x"] = true
	if err := replaceMaterializedView(ctx, ch, "mv_x", func(name string) string {
		return "CREATE MATERIALIZED VIEW " + name + " AS SELECT 1"
	}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(ch.execCalls, " | ")
	if !strings.Contains(joined, "EXCHANGE TABLES") {
		t.Fatalf("want exchange path: %s", joined)
	}
}

func TestEnsureEdgesAggSchemaCreatePath(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() {
		needsSchemaDDLFn = old
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle", Message: "not started"})
	})
	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return true, nil
	}
	ch := newStubConn()
	// tables don't exist → create
	if err := EnsureEdgesAggSchema(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
	if aggstate.GetEdgesAggStatus().State != "pending" {
		t.Fatalf("state=%q", aggstate.GetEdgesAggStatus().State)
	}
}

func TestEnsureEdgesAggSchemaRebuildPath(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() {
		needsSchemaDDLFn = old
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle"})
	})
	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return true, nil
	}
	ch := newStubConn()
	ch.tables[sqlclause.IPEdgesDailyTable] = true
	ch.tables[sqlclause.IPEdgesDailyMV] = true
	ch.colType[sqlclause.IPEdgesDailyTable+".src_ip"] = "String" // rebuild
	ch.partKey[sqlclause.IPEdgesDailyTable] = "day"
	if err := EnsureEdgesAggSchema(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureEdgesAggSchemaTTLOnlyPath(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() {
		needsSchemaDDLFn = old
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle"})
	})
	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return true, nil
	}
	ch := newStubConn()
	ch.tables[sqlclause.IPEdgesDailyTable] = true
	ch.colType[sqlclause.IPEdgesDailyTable+".src_ip"] = "IPv4"
	ch.partKey[sqlclause.IPEdgesDailyTable] = "day"
	ch.colOK[sqlclause.IPEdgesDailyTable+".coord_weight"] = true
	if err := EnsureEdgesAggSchema(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
}

func TestBackfillAndRefreshEdgesAgg(t *testing.T) {
	t.Cleanup(func() {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle"})
	})
	ctx := context.Background()
	ch := newStubConn()
	ch.rows["traffic_logs"] = 0
	if err := BackfillEdgesAgg(ctx, ch); err != nil {
		t.Fatal(err)
	}
	if aggstate.GetEdgesAggStatus().State != "ready" {
		t.Fatalf("state=%q", aggstate.GetEdgesAggStatus().State)
	}

	ch.rows["traffic_logs"] = 100
	ch.rows[sqlclause.IPEdgesDailyTable] = 10
	ch.missingDays = nil
	if err := BackfillEdgesAgg(ctx, ch); err != nil {
		t.Fatal(err)
	}

	day := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	ch.missingDays = []time.Time{day}
	if err := BackfillEdgesAgg(ctx, ch); err != nil {
		t.Fatal(err)
	}
	if aggstate.GetEdgesAggStatus().State != "ready" {
		t.Fatalf("after backfill state=%q", aggstate.GetEdgesAggStatus().State)
	}

	ch.missingDays = nil
	ch.rows["traffic_logs"] = 5
	if err := RefreshEdgesAggReady(ctx, ch); err != nil {
		t.Fatal(err)
	}
	ch.rows["traffic_logs"] = 0
	if err := RefreshEdgesAggReady(ctx, ch); err != nil {
		t.Fatal(err)
	}
}

func TestInsertIPEdgesDaysRejectsBadTable(t *testing.T) {
	err := insertIPEdgesDays(context.Background(), newStubConn(), "evil", nil, nil, false)
	if err == nil {
		t.Fatal("want reject")
	}
}

func TestRebuildIPEdgesDays(t *testing.T) {
	ch := newStubConn()
	ch.tables[sqlclause.IPEdgesDailyTable] = true
	ch.partID[sqlclause.IPEdgesDailyTable] = "20240701"
	day := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	if err := rebuildIPEdgesDays(context.Background(), ch, sqlclause.IPEdgesDailyTable, []time.Time{day}); err != nil {
		t.Fatal(err)
	}
	if err := rebuildIPEdgesDays(context.Background(), ch, "bad!", nil); err == nil {
		t.Fatal("want invalid")
	}
}

func TestEnsureHourlyEdgesAggPaths(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() {
		needsSchemaDDLFn = old
		aggstate.SetHourlyEdgesAggReady(false)
	})
	ctx := context.Background()

	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return true, nil
	}
	ch := newStubConn()
	if err := EnsureHourlyEdgesAggSchema(ctx, ch); err != nil {
		t.Fatal(err)
	}

	ch2 := newStubConn()
	ch2.tables[sqlclause.IPEdgesHourlyTable] = true
	ch2.colType[sqlclause.IPEdgesHourlyTable+".src_ip"] = "IPv4"
	ch2.colOK[sqlclause.IPEdgesHourlyTable+".coord_weight"] = true
	if err := EnsureHourlyEdgesAggSchema(ctx, ch2); err != nil {
		t.Fatal(err)
	}

	ch3 := newStubConn()
	ch3.tables[sqlclause.IPEdgesHourlyTable] = true
	ch3.colType[sqlclause.IPEdgesHourlyTable+".src_ip"] = "String"
	if err := EnsureHourlyEdgesAggSchema(ctx, ch3); err != nil {
		t.Fatal(err)
	}

	ch4 := newStubConn()
	ch4.rows["traffic_logs"] = 0
	ch4.tables[sqlclause.IPEdgesHourlyTable] = true
	if err := BackfillHourlyEdgesAgg(ctx, ch4); err != nil {
		t.Fatal(err)
	}
	ch4.rows["traffic_logs"] = 10
	ch4.missingDays = nil
	if err := BackfillHourlyEdgesAgg(ctx, ch4); err != nil {
		t.Fatal(err)
	}
	ch4.missingDays = []time.Time{time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)}
	if err := BackfillHourlyEdgesAgg(ctx, ch4); err != nil {
		t.Fatal(err)
	}
	if err := RefreshHourlyEdgesAggReady(ctx, ch4); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureGeoEdgesAggPaths(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() {
		needsSchemaDDLFn = old
		aggstate.SetGeoEdgesAggReady(false)
	})
	ctx := context.Background()
	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return true, nil
	}
	ch := newStubConn()
	if err := EnsureGeoEdgesAggSchema(ctx, ch); err != nil {
		t.Fatal(err)
	}

	// existing tables with day partition + enrich ip IPv4
	ch2 := newStubConn()
	for _, gb := range []string{"city", "country", "continent"} {
		tbl := sqlclause.GeoEdgesTable(gb)
		ch2.tables[tbl] = true
		ch2.partKey[tbl] = "day"
		ch2.tables[sqlclause.GeoEdgesMV(gb)] = true
	}
	ch2.tables[sqlclause.GeoEnrichIPTable] = true
	ch2.colType[sqlclause.GeoEnrichIPTable+".ip"] = "IPv4"
	if err := EnsureGeoEdgesAggSchema(ctx, ch2); err != nil {
		t.Fatal(err)
	}

	// enrich table wrong type → drop+recreate
	ch3 := newStubConn()
	ch3.tables[sqlclause.GeoEnrichIPTable] = true
	ch3.colType[sqlclause.GeoEnrichIPTable+".ip"] = "String"
	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return false, nil
	}
	if err := EnsureGeoEdgesAggSchema(ctx, ch3); err != nil {
		t.Fatal(err)
	}

	ch4 := newStubConn()
	ch4.rows["traffic_logs"] = 0
	for _, gb := range []string{"city", "country", "continent"} {
		ch4.tables[sqlclause.GeoEdgesTable(gb)] = true
	}
	if err := BackfillGeoEdgesAgg(ctx, ch4); err != nil {
		t.Fatal(err)
	}
	if !aggstate.PreferGeoEdgesAgg() {
		t.Fatal("want ready after empty backfill")
	}

	ch5 := newStubConn()
	ch5.rows["traffic_logs"] = 50
	for _, gb := range []string{"city", "country", "continent"} {
		ch5.tables[sqlclause.GeoEdgesTable(gb)] = true
		ch5.rows[sqlclause.GeoEdgesTable(gb)] = 1
	}
	ch5.missingDays = nil
	if err := BackfillGeoEdgesAgg(ctx, ch5); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	ch5.missingDays = []time.Time{day}
	if err := BackfillGeoEdgesAgg(ctx, ch5); err != nil {
		t.Fatal(err)
	}
	ch5.missingDays = nil
	if err := RefreshGeoEdgesAggReady(ctx, ch5); err != nil {
		t.Fatal(err)
	}
}

func TestRebuildGeoEdgesLookback(t *testing.T) {
	ctx := context.Background()
	if err := RebuildGeoEdgesLookback(ctx, nil, 3); err != nil {
		t.Fatal(err)
	}
	ch := newStubConn()
	ch.lookbackDays = nil
	ch.missingDays = nil
	if err := RebuildGeoEdgesLookback(ctx, ch, 3); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	ch.lookbackDays = []time.Time{day}
	for _, gb := range []string{"city", "country", "continent"} {
		ch.tables[sqlclause.GeoEdgesTable(gb)] = true
		ch.partID[sqlclause.GeoEdgesTable(gb)] = "20240401"
	}
	ch.tables[sqlclause.IPEdgesDailyTable] = true
	ch.tables[sqlclause.IPEdgesHourlyTable] = true
	ch.partID[sqlclause.IPEdgesDailyTable] = "20240401"
	ch.partID[sqlclause.IPEdgesHourlyTable] = "20240401"
	if err := RebuildGeoEdgesLookback(ctx, ch, 7); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureTrafficLogsIPv4Paths(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() { needsSchemaDDLFn = old })
	ctx := context.Background()

	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return false, nil
	}
	if err := EnsureTrafficLogsIPv4(ctx, newStubConn()); err != nil {
		t.Fatal(err)
	}

	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return true, nil
	}
	// create missing table
	ch := newStubConn()
	if err := EnsureTrafficLogsIPv4(ctx, ch); err != nil {
		t.Fatal(err)
	}

	// already current layout
	ch2 := newStubConn()
	ch2.tables["traffic_logs"] = true
	ch2.colType["traffic_logs.src_ip"] = "IPv4"
	ch2.sortKey["traffic_logs"] = "toStartOfHour(timestamp), src_ip, dst_ip"
	ch2.colType["traffic_logs.src_country"] = "LowCardinality(String)"
	// raw absent
	if err := EnsureTrafficLogsIPv4(ctx, ch2); err != nil {
		t.Fatal(err)
	}

	// rebuild via exchange (String src_ip)
	ch3 := newStubConn()
	ch3.tables["traffic_logs"] = true
	ch3.colType["traffic_logs.src_ip"] = "String"
	ch3.sortKey["traffic_logs"] = "timestamp"
	ch3.colType["traffic_logs.src_country"] = "String"
	ch3.colOK["traffic_logs.raw"] = true
	if err := EnsureTrafficLogsIPv4(ctx, ch3); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureHTTPSchema(t *testing.T) {
	old := needsSchemaDDLFn
	t.Cleanup(func() {
		needsSchemaDDLFn = old
		aggstate.SetGeoEdgesAggReady(false)
	})
	needsSchemaDDLFn = func(context.Context, clickhouse.Conn, string, uint32) (bool, error) {
		return false, nil
	}
	ch := newStubConn()
	// EnsureHTTPSchema still runs EnsureBaseSchema (always) + EnsureTrafficLogsIPv4 (skip) + EnsureGeoEdgesAggSchema
	// geo still calls ensureGeoEnrichIPTable
	if err := EnsureHTTPSchema(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
}

func TestEdgesDailyNeedsRebuildVariants(t *testing.T) {
	ctx := context.Background()
	ch := newStubConn()
	ch.colType["t.src_ip"] = "IPv4"
	ch.partKey["t"] = "toYYYYMM(day)"
	need, reason, err := edgesDailyNeedsRebuild(ctx, ch, "t")
	if err != nil || !need || reason == "" {
		t.Fatalf("need=%v reason=%q err=%v", need, reason, err)
	}
	ch.partKey["t"] = "day"
	// missing coord_weight
	need, reason, err = edgesDailyNeedsRebuild(ctx, ch, "t")
	if err != nil || !need || !strings.Contains(reason, "coord_weight") {
		t.Fatalf("need=%v reason=%q err=%v", need, reason, err)
	}
}

func TestUpdateEdgesBackfillStatus(t *testing.T) {
	t.Cleanup(func() {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle"})
	})
	fn := updateEdgesBackfillStatus(10, 3)
	fn(1)
	st := aggstate.GetEdgesAggStatus()
	if st.DaysDone != 1 || st.DaysTotal != 3 {
		t.Fatalf("%+v", st)
	}
}

func TestEnsureGeoEdgesInvalidGroupBy(t *testing.T) {
	err := ensureGeoEdgesTable(context.Background(), newStubConn(), "nope")
	if err == nil {
		t.Fatal("want invalid groupBy")
	}
	err = rebuildGeoEdgesDays(context.Background(), newStubConn(), "nope", []time.Time{time.Now()})
	if err == nil {
		t.Fatal("want invalid rebuild")
	}
	if err := rebuildGeoEdgesDays(context.Background(), newStubConn(), "city", nil); err != nil {
		t.Fatal(err)
	}
}

func TestGeoEdgesTableRebuildOnBadPartition(t *testing.T) {
	ch := newStubConn()
	tbl := sqlclause.GeoEdgesTable("city")
	ch.tables[tbl] = true
	ch.partKey[tbl] = "toYYYYMM(day)"
	if err := ensureGeoEdgesTable(context.Background(), ch, "city"); err != nil {
		t.Fatal(err)
	}
}
