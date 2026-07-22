package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"network_monitor/internal/adapter/geoipcodec"
	"network_monitor/internal/adapter/httpapi"
	"network_monitor/internal/adapter/parseradapter"
	"network_monitor/internal/auth"
	"network_monitor/internal/config"
	"network_monitor/internal/geoip"
	"network_monitor/internal/ingest"
	"network_monitor/internal/model"
	"network_monitor/internal/parser"
	usecaseauth "network_monitor/internal/usecase/auth"
	usecaseevents "network_monitor/internal/usecase/events"
	usecasegeo "network_monitor/internal/usecase/geo"
	"network_monitor/internal/usecase/parsetest"
	usecasesystem "network_monitor/internal/usecase/system"
)

// In-process smoke после Clean Architecture: auth, map, geo dry-run/upload,
// ingest FeedReader, maintenance backfill. Без Docker/ClickHouse.
func TestPostCASmoke(t *testing.T) {
	hash, err := auth.HashPassword("adminpass1")
	if err != nil {
		t.Fatal(err)
	}
	users, err := auth.NewUserStore(auth.User{
		Username: "admin", PasswordHash: string(hash), Role: auth.RoleAdministrator,
	})
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := auth.NewSessionManager("smoke-session-secret-key-32b!!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	authUC := usecaseauth.New(users, sessions)

	geoIdx := geoip.New()
	geoStore := &memRangeStore{idx: geoIdx}
	var reloadScheduled atomic.Int32
	geoJobs := &stubGeoJobs{reload: &reloadScheduled}
	geoUC := usecasegeo.New(geoStore, &stubMissing{}, geoIdx, geoJobs, geoipcodec.New())

	eventsUC := usecaseevents.New(&stubTraffic{}, geoIdx)

	var maintScheduled atomic.Int32
	systemUC := usecasesystem.New(usecasesystem.Dependencies{
		Maintenance: &stubMaintenance{n: &maintScheduled},
	})
	pinger := stubPinger{}

	parsers := parser.NewRegistry(
		&parser.UserGateCEF{},
		&parser.FortigateCEF{},
		&parser.CiscoFTD{},
		&parser.CiscoASA{},
		&parser.CowrieJSON{},
		&parser.GenericKV{},
	)
	pta := parseradapter.NewParseTest(parsers)
	parseTestUC := parsetest.New(pta, geoIdx, pta)

	ingestSvc := ingest.NewService(ingest.Config{
		QueueSize: 1000, Workers: 1, BatchSize: 100, FlushInterval: time.Second,
	}, ingest.ProcessorDeps{})

	cfg := config.Config{
		ListenAddr:       ":0",
		APIAuthToken:     "smoke-bearer-token",
		MaxLogUploadSize: 1 << 20,
		MaxGeoUploadSize: 1 << 20,
		QueryTimeout:     time.Minute,
		IngestFlushSec:   1,
	}
	srv := httpapi.NewServer(cfg, ingestSvc, eventsUC, geoUC, nil, systemUC, pinger, parseTestUC, nil, authUC, users, sessions)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}

	base := ts.URL
	mustStatus := func(t *testing.T, name string, resp *http.Response, want int) {
		t.Helper()
		if resp.StatusCode != want {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("%s: status=%d want=%d body=%s", name, resp.StatusCode, want, body)
		}
	}

	t.Run("health", func(t *testing.T) {
		resp, err := client.Get(base + "/api/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		mustStatus(t, "health", resp, http.StatusOK)
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["ok"] != true {
			t.Fatalf("health body: %#v", body)
		}
	})

	t.Run("login_me_logout", func(t *testing.T) {
		resp, err := client.Post(base+"/api/auth/login", "application/json",
			strings.NewReader(`{"username":"admin","password":"adminpass1"}`))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		mustStatus(t, "login", resp, http.StatusOK)

		me, err := client.Get(base + "/api/auth/me")
		if err != nil {
			t.Fatal(err)
		}
		defer me.Body.Close()
		mustStatus(t, "me", me, http.StatusOK)
		var user map[string]any
		if err := json.NewDecoder(me.Body).Decode(&user); err != nil {
			t.Fatal(err)
		}
		if user["username"] != "admin" {
			t.Fatalf("me: %#v", user)
		}

		csrf := csrfFromJar(t, jar, base)
		req, err := http.NewRequest(http.MethodPost, base+"/api/auth/logout", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(auth.CSRFHeaderName, csrf)
		req.Header.Set("Origin", base)
		out, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer out.Body.Close()
		mustStatus(t, "logout", out, http.StatusOK)
	})

	// Re-login for protected routes.
	if resp, err := client.Post(base+"/api/auth/login", "application/json",
		strings.NewReader(`{"username":"admin","password":"adminpass1"}`)); err != nil {
		t.Fatal(err)
	} else {
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("re-login %d", resp.StatusCode)
		}
	}
	csrf := csrfFromJar(t, jar, base)

	t.Run("events_map", func(t *testing.T) {
		resp, err := client.Get(base + "/api/events?period=1h&group_by=country")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		mustStatus(t, "events", resp, http.StatusOK)
	})

	const geoCSVHeader = "Network,Country,Region,City,Latitude,Longitude\n"

	t.Run("geo_upload_dry_run", func(t *testing.T) {
		csv := geoCSVHeader + "1.2.3.0/24,RU,Moscow,Moscow,55.75,37.61\n"
		req, err := http.NewRequest(http.MethodPost, base+"/upload-geo?dry_run=1", strings.NewReader(csv))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "text/csv")
		req.Header.Set(auth.CSRFHeaderName, csrf)
		req.Header.Set("Origin", base)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		mustStatus(t, "geo dry-run", resp, http.StatusOK)
	})

	t.Run("geo_upload_apply", func(t *testing.T) {
		csv := geoCSVHeader + "8.8.8.0/24,US,CA,Mountain View,37.4,-122.1\n"
		req, err := http.NewRequest(http.MethodPost, base+"/upload-geo", strings.NewReader(csv))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "text/csv")
		req.Header.Set(auth.CSRFHeaderName, csrf)
		req.Header.Set("Origin", base)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		mustStatus(t, "geo upload", resp, http.StatusOK)
		if reloadScheduled.Load() < 1 {
			t.Fatal("expected ScheduleReloadAndEnrich after geo upload")
		}
		if geoIdx.RangeCount() < 1 {
			t.Fatal("expected ranges in geo index")
		}
	})

	t.Run("ingest", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, base+"/api/ingest",
			strings.NewReader("src=1.1.1.1 dst=8.8.8.8 action=allow proto=tcp\n"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(auth.CSRFHeaderName, csrf)
		req.Header.Set("Origin", base)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		mustStatus(t, "ingest", resp, http.StatusOK)
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["ok"] != true {
			t.Fatalf("ingest body: %#v", body)
		}
	})

	t.Run("parse_test", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, base+"/api/parse-test",
			strings.NewReader("src=10.0.0.1 dst=10.0.0.2 action=deny\n"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(auth.CSRFHeaderName, csrf)
		req.Header.Set("Origin", base)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		mustStatus(t, "parse-test", resp, http.StatusOK)
	})

	t.Run("maintenance_backfill", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, base+"/api/system/maintenance/backfill", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(auth.CSRFHeaderName, csrf)
		req.Header.Set("Origin", base)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		mustStatus(t, "maintenance", resp, http.StatusAccepted)
		if maintScheduled.Load() < 1 {
			t.Fatal("expected ScheduleMaintenanceBackfill via systemUC")
		}
	})

	t.Run("ingest_stats_bearer", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, base+"/api/ingest/stats", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer smoke-bearer-token")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		mustStatus(t, "ingest stats", resp, http.StatusOK)
	})
}

func csrfFromJar(t *testing.T, jar http.CookieJar, base string) string {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range jar.Cookies(u) {
		if c.Name == auth.CSRFCookieName && c.Value != "" {
			return c.Value
		}
	}
	t.Fatal("csrf cookie missing")
	return ""
}

type stubPinger struct{}

func (stubPinger) Ping(context.Context) error { return nil }

type stubMaintenance struct{ n *atomic.Int32 }

func (s *stubMaintenance) ScheduleMaintenanceBackfill(context.Context, time.Duration) {
	s.n.Add(1)
}

type stubGeoJobs struct{ reload *atomic.Int32 }

func (s *stubGeoJobs) ScheduleReloadAndEnrich(context.Context, time.Duration) {
	s.reload.Add(1)
}

type stubTraffic struct{}

func (stubTraffic) ScanRawAggsForTimeRange(context.Context, model.TimeRange, int, string, time.Duration) ([]model.RawAgg, error) {
	return nil, nil
}
func (stubTraffic) ScanGeoEdgesForTimeRange(context.Context, model.TimeRange, string, int, string, time.Duration) ([]model.GeoEdgeAgg, bool, error) {
	return nil, false, nil
}

type stubMissing struct{}

func (stubMissing) ScanGeoMissingIPsForTimeRange(context.Context, model.TimeRange, int, time.Duration) ([]model.GeoMissingIPRow, error) {
	return nil, nil
}

type memRangeStore struct {
	idx    *geoip.Index
	ranges []model.GeoRange
}

func (m *memRangeStore) Replace(_ context.Context, ranges []model.GeoRange) (int, error) {
	m.ranges = append([]model.GeoRange(nil), ranges...)
	if m.idx != nil {
		m.idx.ReplaceRanges(ranges)
	}
	return len(ranges), nil
}
func (m *memRangeStore) Load(context.Context) ([]model.GeoRange, error) {
	return append([]model.GeoRange(nil), m.ranges...), nil
}
func (m *memRangeStore) Count(context.Context) (int, error) { return len(m.ranges), nil }
func (m *memRangeStore) FindByIP(context.Context, string) (model.GeoRange, bool, error) {
	return model.GeoRange{}, false, nil
}
func (m *memRangeStore) ListPage(_ context.Context, limit int, _ string) ([]model.GeoRange, int, int, bool, error) {
	n := len(m.ranges)
	if limit <= 0 || limit > n {
		limit = n
	}
	return append([]model.GeoRange(nil), m.ranges[:limit]...), n, n, false, nil
}
