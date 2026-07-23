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

func (s *stubRepo) ScanRawAggsForTimeRange(ctx context.Context, tr model.TimeRange, limit int, filter string, timeout time.Duration) ([]model.RawAgg, error) {
	return s.raws, nil
}

func (s *stubRepo) ScanGeoEdgesForTimeRange(ctx context.Context, tr model.TimeRange, groupBy string, limit int, filter string, timeout time.Duration) ([]model.GeoEdgeAgg, bool, error) {
	return s.geoRows, s.geoOK, nil
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
		geoOK: true,
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
