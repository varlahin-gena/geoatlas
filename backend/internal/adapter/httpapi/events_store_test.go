package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"network_monitor/internal/config"
	"network_monitor/internal/geoip"
	"network_monitor/internal/model"
	usecaseevents "network_monitor/internal/usecase/events"
)

type stubTraffic struct {
	geoCalled bool
	geoMode   string
	rawCalled bool
	lastQuery usecaseevents.MapScanQuery
}

func (s *stubTraffic) ScanMapAggs(ctx context.Context, tr model.TimeRange, q usecaseevents.MapScanQuery, timeout time.Duration) (usecaseevents.MapAggScanResult, error) {
	s.geoCalled = true
	s.geoMode = tr.Mode
	s.lastQuery = q
	return usecaseevents.MapAggScanResult{
		Source: "geo_" + q.GroupBy,
		GeoEdges: []model.GeoEdgeAgg{{
			SrcKey: "Moscow, Россия", DstKey: "Berlin, Germany",
			SrcLabel: "Moscow", DstLabel: "Berlin",
			SrcLat: 55.75, SrcLon: 37.62, DstLat: 52.52, DstLon: 13.40,
			Count: 3, AllowedCnt: 3,
		}},
	}, nil
}

func (s *stubTraffic) ScanCountrySeries(ctx context.Context, tr model.TimeRange, country, dataSource string, timeout time.Duration) ([]usecaseevents.SeriesPoint, int, error) {
	return nil, 3600, nil
}

func TestGetEventsUsesTrafficStoreForHoursGeo(t *testing.T) {
	stub := &stubTraffic{}
	h := &EventsHandler{EventsDeps: &EventsDeps{
		cfg:      config.Config{QueryTimeout: time.Minute},
		eventsUC: usecaseevents.New(stub, geoip.New(), nil),
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/events?hours=6&group_by=city", nil)
	rec := httptest.NewRecorder()
	h.GetEvents(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !stub.geoCalled {
		t.Fatal("expected ScanGeoEdgesForTimeRange")
	}
	if stub.geoMode != "hours" {
		t.Fatalf("geo mode = %q, want hours", stub.geoMode)
	}
	if stub.rawCalled {
		t.Fatal("should not fall back to raw IP path when geo ok")
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	stats := body["stats"].(map[string]any)
	if stats["source"] != "geo_city" {
		t.Fatalf("source = %v", stats["source"])
	}
}

func TestGetEventsIPUsesGeoEdgesPath(t *testing.T) {
	stub := &stubTraffic{}
	h := &EventsHandler{EventsDeps: &EventsDeps{
		cfg:      config.Config{QueryTimeout: time.Minute},
		eventsUC: usecaseevents.New(stub, geoip.New(), nil),
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/events?hours=6&group_by=ip", nil)
	rec := httptest.NewRecorder()
	h.GetEvents(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !stub.geoCalled {
		t.Fatal("expected ScanGeoEdgesForTimeRange for group_by=ip")
	}
	if stub.rawCalled {
		t.Fatal("should not fall back to raw when geo edges ok")
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	stats := body["stats"].(map[string]any)
	if stats["source"] != "geo_ip" {
		t.Fatalf("source = %v, want geo_ip", stats["source"])
	}
	lines, _ := body["lines"].([]any)
	if len(lines) == 0 {
		t.Fatal("expected lines from geo_ip path")
	}
}

type stubTrafficNoGeo struct {
	stubTraffic
}

func (s *stubTrafficNoGeo) ScanMapAggs(ctx context.Context, tr model.TimeRange, q usecaseevents.MapScanQuery, timeout time.Duration) (usecaseevents.MapAggScanResult, error) {
	s.geoCalled = true
	s.geoMode = tr.Mode
	s.rawCalled = true
	return usecaseevents.MapAggScanResult{
		Source: "ip_live_" + q.GroupBy,
		Raws: []model.RawAgg{{
			SrcIP: "1.1.1.1", DstIP: "8.8.8.8",
			Count: 5, AllowedCnt: 5,
		}},
	}, nil
}

func TestGetEventsFallsBackToLiveGeoWhenStoredCoordsEmpty(t *testing.T) {
	stub := &stubTrafficNoGeo{}
	h := &EventsHandler{EventsDeps: &EventsDeps{
		cfg:      config.Config{QueryTimeout: time.Minute},
		eventsUC: usecaseevents.New(stub, geoip.New(), nil),
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/events?hours=24&group_by=city", nil)
	rec := httptest.NewRecorder()
	h.GetEvents(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !stub.geoCalled {
		t.Fatal("expected geo path first")
	}
	if !stub.rawCalled {
		t.Fatal("expected fallback to raw IP path")
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	stats := body["stats"].(map[string]any)
	if stats["source"] != "ip_live_city" {
		t.Fatalf("source = %v, want ip_live_city", stats["source"])
	}
}

type stubTrafficIPLogGeo struct {
	stubTraffic
}

func (s *stubTrafficIPLogGeo) ScanMapAggs(ctx context.Context, tr model.TimeRange, q usecaseevents.MapScanQuery, timeout time.Duration) (usecaseevents.MapAggScanResult, error) {
	s.rawCalled = true
	return usecaseevents.MapAggScanResult{
		Source: "ip_live_" + q.GroupBy,
		Raws: []model.RawAgg{{
			SrcIP: "10.0.0.1", DstIP: "10.0.0.2",
			Count: 7, AllowedCnt: 7,
			SrcCountry: "Russia", DstCountry: "Germany",
			SrcLat: 55.75, SrcLon: 37.62,
			DstLat: 52.52, DstLon: 13.40,
		}},
	}, nil
}

func TestGetEventsIPUsesStoredLogGeoWhenLiveMisses(t *testing.T) {
	stub := &stubTrafficIPLogGeo{}
	h := &EventsHandler{EventsDeps: &EventsDeps{
		cfg:      config.Config{QueryTimeout: time.Minute},
		eventsUC: usecaseevents.New(stub, geoip.New(), nil),
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/events?hours=6&group_by=ip", nil)
	rec := httptest.NewRecorder()
	h.GetEvents(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !stub.rawCalled {
		t.Fatal("expected raw IP path")
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	lines, ok := body["lines"].([]any)
	if !ok || len(lines) == 0 {
		t.Fatalf("expected lines from log geo fallback, body=%s", rec.Body.String())
	}
	stats := body["stats"].(map[string]any)
	if stats["edges"].(float64) < 1 {
		t.Fatalf("edges = %v", stats["edges"])
	}
}

type stubTrafficIPCountryOnly struct {
	stubTraffic
}

func (s *stubTrafficIPCountryOnly) ScanMapAggs(ctx context.Context, tr model.TimeRange, q usecaseevents.MapScanQuery, timeout time.Duration) (usecaseevents.MapAggScanResult, error) {
	s.rawCalled = true
	return usecaseevents.MapAggScanResult{
		Source: "ip_live_" + q.GroupBy,
		Raws: []model.RawAgg{{
			SrcIP: "10.0.0.1", DstIP: "10.0.0.2",
			Count: 7, AllowedCnt: 7,
			SrcCountry: "Russia", DstCountry: "Germany",
		}},
	}, nil
}

func TestGetEventsSubnetUsesCountryFallback(t *testing.T) {
	stub := &stubTrafficIPCountryOnly{}
	h := &EventsHandler{EventsDeps: &EventsDeps{
		cfg:      config.Config{QueryTimeout: time.Minute},
		eventsUC: usecaseevents.New(stub, geoip.New(), nil),
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/events?hours=6&group_by=subnet", nil)
	rec := httptest.NewRecorder()
	h.GetEvents(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	lines, _ := body["lines"].([]any)
	if len(lines) == 0 {
		t.Fatalf("expected subnet lines via country center, body=%s", rec.Body.String())
	}
}

func TestGetEventsPassesFilterCountryAndQ(t *testing.T) {
	stub := &stubTraffic{}
	h := &EventsHandler{EventsDeps: &EventsDeps{
		cfg:      config.Config{QueryTimeout: time.Minute},
		eventsUC: usecaseevents.New(stub, geoip.New(), nil),
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/events?hours=6&group_by=city&filter=blocked&country=Russia&q=tcp&limit=5000", nil)
	rec := httptest.NewRecorder()
	h.GetEvents(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if stub.lastQuery.Filter != "blocked" || stub.lastQuery.Country != "Russia" || stub.lastQuery.Query != "tcp" || stub.lastQuery.Limit != 5000 {
		t.Fatalf("query = %#v", stub.lastQuery)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["filter"] != "blocked" || body["country"] != "Russia" || body["q"] != "tcp" {
		t.Fatalf("echo body=%v", body)
	}
}

func TestGetEventsPassesAdvancedQAndReputation(t *testing.T) {
	stub := &stubTraffic{}
	h := &EventsHandler{EventsDeps: &EventsDeps{
		cfg:      config.Config{QueryTimeout: time.Minute},
		eventsUC: usecaseevents.New(stub, geoip.New(), nil),
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/events?hours=6&group_by=ip&q=country:Germany+AND+rule:block&rep_cat=malware&rep_list=spamhaus&rep_side=src&limit=100", nil)
	rec := httptest.NewRecorder()
	h.GetEvents(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if stub.lastQuery.Query != "country:Germany AND rule:block" {
		t.Fatalf("q = %q", stub.lastQuery.Query)
	}
	if stub.lastQuery.Limit != 50000 {
		t.Fatalf("scan limit = %d, want hard max when reputation filter is on", stub.lastQuery.Limit)
	}
}
