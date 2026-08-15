package events

import (
	"context"
	"testing"
	"time"

	"network_monitor/internal/model"
)

type stubRepo struct {
	geoRows []model.GeoEdgeAgg
	geoOK   bool
	raws    []model.RawAgg
}

func (s *stubRepo) ScanMapAggs(ctx context.Context, tr model.TimeRange, q MapScanQuery, timeout time.Duration) (MapAggScanResult, error) {
	if s.geoOK {
		return MapAggScanResult{Source: "geo_" + q.GroupBy, GeoEdges: s.geoRows}, nil
	}
	return MapAggScanResult{Source: "ip_live_" + q.GroupBy, Raws: s.raws}, nil
}

func (s *stubRepo) ScanCountrySeries(ctx context.Context, tr model.TimeRange, country, dataSource string, timeout time.Duration) ([]SeriesPoint, int, error) {
	return nil, 3600, nil
}

type stubGeo map[string]model.GeoLookup

func (s stubGeo) Lookup(ipStr string) model.GeoLookup {
	if lk, ok := s[ipStr]; ok {
		return lk
	}
	return model.GeoLookup{}
}

func TestGetMapUsesGeoEdges(t *testing.T) {
	repo := &stubRepo{
		geoOK: true,
		geoRows: []model.GeoEdgeAgg{{
			SrcKey: "A", DstKey: "B", SrcLabel: "A", DstLabel: "B",
			SrcLat: 1, SrcLon: 2, DstLat: 3, DstLon: 4,
			Count: 2, AllowedCnt: 2,
		}},
	}
	uc := New(repo, stubGeo{}, nil)
	out, err := uc.GetMap(context.Background(), GetMapInput{
		TimeRange: model.TimeRange{Mode: "hours", Amount: 6},
		Limit:     100,
		GroupBy:   "city",
		Filter:    "all",
		Timeout:   time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Source != "geo_city" {
		t.Fatalf("source=%q", out.Source)
	}
	if len(out.Lines) == 0 {
		t.Fatal("expected lines")
	}
}

func TestGetMapFallsBackWhenGeoEmpty(t *testing.T) {
	repo := &stubRepo{
		geoOK: false,
		geoRows: []model.GeoEdgeAgg{{
			SrcKey: "city:unknown", DstKey: "city:unknown",
			SrcLabel: "Неизвестно", DstLabel: "Неизвестно",
			Count: 10, AllowedCnt: 10,
		}},
		raws: []model.RawAgg{{
			SrcIP: "1.1.1.1", DstIP: "8.8.8.8", Count: 5, AllowedCnt: 5,
		}},
	}
	geo := stubGeo{
		"1.1.1.1": {Found: true, Lat: 1, Lon: 1, City: "S", Country: "AU"},
		"8.8.8.8": {Found: true, Lat: 2, Lon: 2, City: "M", Country: "US"},
	}
	uc := New(repo, geo, nil)
	out, err := uc.GetMap(context.Background(), GetMapInput{
		TimeRange: model.TimeRange{Mode: "hours", Amount: 6},
		GroupBy:   "city",
		Filter:    "all",
		Timeout:   time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Source != "ip_live_city" {
		t.Fatalf("source=%q want ip_live_city", out.Source)
	}
}

// typedNilRep mimics wire passing (*T)(nil) into ReputationLookuper.
// Lookup намеренно НЕ nil-safe: enrichment должен отсечь typed-nil до вызова.
type typedNilRep struct{ marker int }

func (t *typedNilRep) Lookup(ipStr string) []model.ReputationHit {
	_ = t.marker // panic if called on typed-nil
	return []model.ReputationHit{{List: "x", Category: "y"}}
}

func TestGetMapIPModeWithTypedNilReputation(t *testing.T) {
	repo := &stubRepo{
		raws: []model.RawAgg{{
			SrcIP: "1.1.1.1", DstIP: "8.8.8.8", Count: 5, AllowedCnt: 5,
		}},
	}
	geo := stubGeo{
		"1.1.1.1": {Found: true, Lat: 1, Lon: 1, City: "S", Country: "AU"},
		"8.8.8.8": {Found: true, Lat: 2, Lon: 2, City: "M", Country: "US"},
	}
	var nilRep *typedNilRep
	var lookuper ReputationLookuper = nilRep // typed nil in interface (SA4023: never == nil)
	uc := New(repo, geo, lookuper)
	out, err := uc.GetMap(context.Background(), GetMapInput{
		TimeRange: model.TimeRange{Mode: "hours", Amount: 6},
		GroupBy:   "ip",
		Filter:    "all",
		Timeout:   time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Lines) == 0 {
		t.Fatal("expected lines")
	}
	for _, ln := range out.Lines {
		if len(ln.SrcReputation) != 0 || len(ln.DstReputation) != 0 {
			t.Fatalf("typed-nil lookuper must skip enrich, got %#v", ln)
		}
	}
}

type stubRep map[string][]model.ReputationHit

func (s stubRep) Lookup(ipStr string) []model.ReputationHit {
	return s[ipStr]
}

func TestGetMapFiltersReputationBeforeLimit(t *testing.T) {
	repo := &stubRepo{
		raws: []model.RawAgg{
			{SrcIP: "1.1.1.1", DstIP: "8.8.8.8", Count: 50, AllowedCnt: 50},
			{SrcIP: "2.2.2.2", DstIP: "9.9.9.9", Count: 5, AllowedCnt: 5},
		},
	}
	geo := stubGeo{
		"1.1.1.1": {Found: true, Lat: 1, Lon: 1, City: "S", Country: "AU"},
		"8.8.8.8": {Found: true, Lat: 2, Lon: 2, City: "M", Country: "US"},
		"2.2.2.2": {Found: true, Lat: 3, Lon: 3, City: "A", Country: "DE"},
		"9.9.9.9": {Found: true, Lat: 4, Lon: 4, City: "B", Country: "FR"},
	}
	rep := stubRep{
		"2.2.2.2": {{List: "spamhaus", Category: "malware"}},
	}
	uc := New(repo, geo, rep)
	out, err := uc.GetMap(context.Background(), GetMapInput{
		TimeRange:     model.TimeRange{Mode: "hours", Amount: 6},
		GroupBy:       "ip",
		Filter:        "all",
		Limit:         1,
		RepCategories: []string{"malware"},
		Timeout:       time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Lines) != 1 {
		t.Fatalf("lines=%d", len(out.Lines))
	}
	if out.Lines[0].Src != "2.2.2.2" {
		t.Fatalf("filtered line src=%s", out.Lines[0].Src)
	}
	if _, ok := out.ReputationFacets["malware"]; !ok {
		t.Fatalf("facets=%v", out.ReputationFacets)
	}
}
