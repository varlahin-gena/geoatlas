//go:build integration

package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	chadapter "network_monitor/internal/adapter/clickhouse"
	"network_monitor/internal/adapter/clickhouse/aggstate"
	"network_monitor/internal/adapter/clickhouse/ingeststore"
	"network_monitor/internal/adapter/clickhouse/trafficstore"
	"network_monitor/internal/adapter/geoipcodec"
	httpapi "network_monitor/internal/adapter/httpapi"
	"network_monitor/internal/adapter/parseradapter"
	"network_monitor/internal/config"
	"network_monitor/internal/geoip"
	"network_monitor/internal/adapter/ingestnet"
	"network_monitor/internal/parser"
	usecaseevents "network_monitor/internal/usecase/events"
	usecasegeo "network_monitor/internal/usecase/geo"
)

const mapPathBearer = "e2e-map-path-token"

func TestIntegrationMapPathLogToEvents(t *testing.T) {
	conn := mapPathCH(t)
	mapPathSchema(t, conn)
	aggstate.SetGeoEdgesAggReady(false)
	aggstate.SetHourlyEdgesAggReady(false)
	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle", Message: "map-path e2e"})
	t.Cleanup(func() {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle", Message: "not started"})
	})

	geoIdx := geoip.New()
	geoStore := &memRangeStore{idx: geoIdx}
	geoUC := usecasegeo.New(geoStore, &stubMissing{}, geoIdx, &stubGeoJobs{}, geoipcodec.New(), 100_000)

	parsers := parser.NewRegistry(
		&parser.UserGateCEF{},
		&parser.FortigateCEF{},
		&parser.CiscoFTD{},
		&parser.CiscoASA{},
		&parser.CowrieJSON{},
		&parser.GenericKV{},
	)
	ingestRepo := ingeststore.NewIngestRepository(conn)
	ingestSvc := ingestnet.NewService(ingestnet.Config{
		QueueSize:     10_000,
		Workers:       1,
		BatchSize:     32,
		FlushInterval: 50 * time.Millisecond,
		QueryTimeout:  15 * time.Second,
	}, ingestnet.ProcessorDeps{
		Logs: ingestRepo, Errors: ingestRepo,
		Parser: parseradapter.New(parsers), Geo: geoIdx, EnrichCountry: true,
	})

	runCtx, runCancel := context.WithCancel(context.Background())
	t.Cleanup(runCancel)
	go func() { _ = ingestSvc.Run(runCtx) }()
	waitIngestRunning(t, ingestSvc)

	eventsUC := usecaseevents.New(trafficstore.NewTrafficRepository(conn), geoIdx, nil)
	cfg := config.Config{
		ListenAddr:         ":0",
		APIAuthToken:       mapPathBearer,
		MaxLogUploadSize:   1 << 20,
		MaxGeoUploadSize:   1 << 20,
		MaxGeoUploadRanges: 100_000,
		QueryTimeout:       30 * time.Second,
		IngestFlushSec:     1,
	}
	srv := httpapi.NewServer(httpapi.Params{
		Cfg:      cfg,
		Ingest:   ingestSvc,
		EventsUC: eventsUC,
		GeoUC:    geoUC,
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	client := &http.Client{Timeout: 20 * time.Second}
	bearer := func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+mapPathBearer)
	}

	geoCSV := "Network,Country,Region,City,Latitude,Longitude\n" +
		"10.0.0.0/8,Testland,Lab,LabCity,55.75,37.62\n" +
		"192.0.2.0/24,Testland,Lab,LabCity,55.75,37.62\n" +
		"198.51.100.0/24,Testland,Lab,LabCity,55.76,37.63\n"
	geoReq, err := http.NewRequest(http.MethodPost, ts.URL+"/upload-geo", strings.NewReader(geoCSV))
	if err != nil {
		t.Fatal(err)
	}
	geoReq.Header.Set("Content-Type", "text/csv")
	bearer(geoReq)
	geoResp, err := client.Do(geoReq)
	if err != nil {
		t.Fatal(err)
	}
	defer geoResp.Body.Close()
	if geoResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(geoResp.Body)
		t.Fatalf("upload-geo: status=%d body=%s", geoResp.StatusCode, body)
	}
	if geoIdx.RangeCount() < 1 {
		t.Fatal("geo index empty after upload-geo")
	}

	var logs strings.Builder
	for _, s := range parser.Samples() {
		if s.Skip {
			continue
		}
		logs.WriteString(s.Line)
		logs.WriteByte('\n')
	}
	logReq, err := http.NewRequest(http.MethodPost, ts.URL+"/upload-logs", strings.NewReader(logs.String()))
	if err != nil {
		t.Fatal(err)
	}
	logReq.Header.Set("Content-Type", "text/plain")
	bearer(logReq)
	logResp, err := client.Do(logReq)
	if err != nil {
		t.Fatal(err)
	}
	defer logResp.Body.Close()
	if logResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(logResp.Body)
		t.Fatalf("upload-logs: status=%d body=%s", logResp.StatusCode, body)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		st := ingestSvc.Stats()
		if st.InsertedTotal >= 1 && st.QueueDepth == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("ingest did not insert: %+v", st)
		}
		time.Sleep(50 * time.Millisecond)
	}

	evReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/events?hours=1&group_by=ip", nil)
	if err != nil {
		t.Fatal(err)
	}
	bearer(evReq)
	evResp, err := client.Do(evReq)
	if err != nil {
		t.Fatal(err)
	}
	defer evResp.Body.Close()
	if evResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(evResp.Body)
		t.Fatalf("events: status=%d body=%s", evResp.StatusCode, body)
	}
	var ev struct {
		Points map[string]json.RawMessage `json:"points"`
		Lines  []json.RawMessage          `json:"lines"`
		Stats  struct {
			Nodes    int    `json:"nodes"`
			Edges    int    `json:"edges"`
			Skipped  int    `json:"skipped_no_geo"`
			Source   string `json:"source"`
			RawPairs int    `json:"raw_pairs"`
		} `json:"stats"`
	}
	if err := json.NewDecoder(evResp.Body).Decode(&ev); err != nil {
		t.Fatal(err)
	}
	if ev.Stats.Nodes < 1 || ev.Stats.Edges < 1 || len(ev.Points) < 1 || len(ev.Lines) < 1 {
		t.Fatalf("map empty after ingest: stats=%+v points=%d lines=%d", ev.Stats, len(ev.Points), len(ev.Lines))
	}
}

func waitIngestRunning(t *testing.T, svc *ingestnet.Service) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if svc.Stats().State == "running" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("ingest not running: %+v", svc.Stats())
}

func mapPathCH(t *testing.T) ch.Conn {
	t.Helper()
	addr := os.Getenv("CLICKHOUSE_TEST_ADDR")
	if addr == "" {
		t.Skip("CLICKHOUSE_TEST_ADDR not set")
	}
	user := os.Getenv("CLICKHOUSE_TEST_USER")
	if user == "" {
		user = "default"
	}
	db := os.Getenv("CLICKHOUSE_TEST_DB")
	if db == "" {
		db = "default"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	conn, err := chadapter.ConnectWithPool(ctx, addr, chadapter.Auth{
		Username: user,
		Password: os.Getenv("CLICKHOUSE_TEST_PASSWORD"),
		Database: db,
	}, chadapter.PoolOptions{Name: "map-path-e2e", MaxOpenConns: 4, MaxIdleConns: 2})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func mapPathSchema(t *testing.T, conn ch.Conn) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS traffic_logs (
			timestamp DateTime64(3),
			parsed_at DateTime64(3) DEFAULT now64(3),
			vendor LowCardinality(String) DEFAULT '',
			device String,
			src_ip IPv4, dst_ip IPv4,
			src_port UInt32, dst_port UInt32,
			action LowCardinality(String),
			rule String, proto LowCardinality(String),
			src_zone String, dst_zone String,
			src_country String, dst_country String,
			src_city String DEFAULT '', dst_city String DEFAULT '',
			src_region String DEFAULT '', dst_region String DEFAULT '',
			src_lat Float64 DEFAULT 0, src_lon Float64 DEFAULT 0,
			dst_lat Float64 DEFAULT 0, dst_lon Float64 DEFAULT 0,
			bytes_sent UInt64, bytes_recv UInt64,
			packets_sent UInt64, packets_recv UInt64,
			raw String
		) ENGINE = MergeTree() ORDER BY (timestamp, src_ip, dst_ip)`,
		`CREATE TABLE IF NOT EXISTS parse_errors (
			id UUID DEFAULT generateUUIDv4(),
			timestamp DateTime64(3) DEFAULT now64(3),
			vendor LowCardinality(String) DEFAULT '',
			reason String,
			raw String
		) ENGINE = MergeTree() ORDER BY timestamp`,
		`TRUNCATE TABLE traffic_logs`,
		`TRUNCATE TABLE parse_errors`,
	}
	for _, s := range stmts {
		if err := conn.Exec(ctx, s); err != nil {
			t.Fatalf("schema: %v\nSQL: %s", err, s)
		}
	}
}
