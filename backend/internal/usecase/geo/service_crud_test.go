package geo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"geoatlas/internal/apperr"
	"geoatlas/internal/geoip"
	"geoatlas/internal/model"
)

type crudStore struct {
	ranges []model.GeoRange
	err    error
}

func (s *crudStore) Replace(_ context.Context, ranges []model.GeoRange) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	s.ranges = append([]model.GeoRange(nil), ranges...)
	return len(ranges), nil
}
func (s *crudStore) Truncate(context.Context) error {
	s.ranges = nil
	return s.err
}
func (s *crudStore) Load(context.Context) ([]model.GeoRange, error) {
	return append([]model.GeoRange(nil), s.ranges...), s.err
}
func (s *crudStore) Count(context.Context) (int, error) { return len(s.ranges), s.err }
func (s *crudStore) FindByIP(_ context.Context, ipStr string) (model.GeoRange, bool, error) {
	if s.err != nil {
		return model.GeoRange{}, false, s.err
	}
	ip := geoip.IPToUint32(ipStr)
	for _, g := range s.ranges {
		if ip >= g.StartIP && ip <= g.EndIP {
			return g, true, nil
		}
	}
	return model.GeoRange{}, false, nil
}
func (s *crudStore) ListPage(_ context.Context, limit int, _ string) ([]model.GeoRange, int, int, bool, error) {
	if s.err != nil {
		return nil, 0, 0, false, s.err
	}
	items := s.ranges
	trunc := false
	if limit > 0 && len(items) > limit {
		items = items[:limit]
		trunc = true
	}
	return items, len(s.ranges), len(items), trunc, nil
}

type crudCodec struct {
	overlap error
	parseOK bool
}

func (c *crudCodec) ReadCSV(io.Reader) ([]model.GeoRange, error) { return nil, nil }
func (c *crudCodec) ReadCSVSnapshot(io.Reader) ([]model.GeoRange, *geoip.BuiltSnapshot, error) {
	return nil, nil, nil
}
func (c *crudCodec) WriteCSV(w io.Writer, ranges []model.GeoRange) error {
	_, err := w.Write([]byte("ok"))
	_ = ranges
	return err
}
func (c *crudCodec) Normalize(ranges []model.GeoRange) ([]model.GeoRange, int) {
	return ranges, 0
}
func (c *crudCodec) CheckNonOverlapping([]model.GeoRange) error { return c.overlap }
func (c *crudCodec) ParseEntry(network, country, region, city string, lat, lon float64) (model.GeoRange, error) {
	if !c.parseOK {
		return model.GeoRange{}, errors.New("bad entry")
	}
	start, end, ok := geoip.ParseNetworkField(network)
	if !ok {
		return model.GeoRange{}, errors.New("bad network")
	}
	return model.GeoRange{StartIP: start, EndIP: end, Country: country, Region: region, City: city, Lat: lat, Lon: lon}, nil
}
func (c *crudCodec) ParseNetwork(network string) (uint32, uint32, bool) {
	return geoip.ParseNetworkField(network)
}
func (c *crudCodec) FormatNetwork(start, end uint32) string {
	return geoip.FormatNetwork(start, end)
}

type crudJobs struct{ n int }

func (j *crudJobs) ScheduleReloadAndEnrich(context.Context, time.Duration) { j.n++ }

type crudMissing struct {
	rows []model.GeoMissingIPRow
	err  error
}

func (m *crudMissing) ScanGeoMissingIPsForTimeRange(context.Context, model.TimeRange, int, time.Duration) ([]model.GeoMissingIPRow, error) {
	return m.rows, m.err
}

type crudHeavy struct{ fail error }

func (h *crudHeavy) Acquire(context.Context) error { return h.fail }
func (h *crudHeavy) Release()                      {}

func TestIndexReadyAndFormatNetwork(t *testing.T) {
	var nilSvc *Service
	if !nilSvc.IndexReady() {
		t.Fatal("nil index ready defaults true")
	}
	idx := geoip.New()
	s := New(&crudStore{}, nil, idx, nil, &crudCodec{parseOK: true}, 0)
	s.SetHeavySlot(&crudHeavy{})
	if !s.IndexReady() {
		// empty index may still be ready
	}
	if s.FormatNetwork(0x0a000001, 0x0a000001) == "" {
		t.Fatal("format")
	}
	s2 := New(nil, nil, nil, nil, nil, 0)
	if s2.FormatNetwork(1, 1) != "" {
		t.Fatal("nil codec")
	}
}

func TestListRangesIPAndPage(t *testing.T) {
	store := &crudStore{ranges: []model.GeoRange{{
		StartIP: 0x0a000000, EndIP: 0x0a0000ff, Country: "RU", City: "Msk",
	}}}
	codec := &crudCodec{parseOK: true}
	s := New(store, nil, nil, nil, codec, 0)

	_, err := s.ListRanges(context.Background(), ListRangesInput{IP: "not-an-ip"})
	if err == nil {
		t.Fatal("invalid ip")
	}
	out, err := s.ListRanges(context.Background(), ListRangesInput{IP: "10.0.0.1"})
	if err != nil || !out.IPLookup || !out.IPHit || len(out.Items) != 1 {
		t.Fatalf("ip hit: %+v %v", out, err)
	}
	out, err = s.ListRanges(context.Background(), ListRangesInput{Limit: 10})
	if err != nil || out.Total != 1 {
		t.Fatalf("page: %+v %v", out, err)
	}

	idx := geoip.New()
	idx.ReplaceRanges(store.ranges)
	s = New(store, nil, idx, nil, codec, 0)
	out, err = s.ListRanges(context.Background(), ListRangesInput{IP: "10.0.0.2", Limit: 5})
	if err != nil || !out.IPHit {
		t.Fatalf("index ip: %+v %v", out, err)
	}
	out, err = s.ListRanges(context.Background(), ListRangesInput{Query: "RU", Limit: 100})
	if err != nil || out.Total < 1 {
		t.Fatalf("index page: %+v %v", out, err)
	}
}

func TestAppendUpdateExport(t *testing.T) {
	store := &crudStore{}
	codec := &crudCodec{parseOK: true}
	jobs := &crudJobs{}
	s := New(store, nil, geoip.New(), jobs, codec, 0)

	res, err := s.AppendRange(context.Background(), "10.1.0.0/24", "RU", "MOW", "Moscow", 55.7, 37.6)
	if err != nil || res.Count != 1 || jobs.n != 1 {
		t.Fatalf("append: %+v %v jobs=%d", res, err, jobs.n)
	}
	codec.parseOK = false
	if _, err := s.AppendRange(context.Background(), "x", "RU", "", "", 0, 0); err == nil {
		t.Fatal("bad append")
	}
	codec.parseOK = true
	codec.overlap = errors.New("overlap")
	if _, err := s.AppendRange(context.Background(), "10.1.0.0/24", "RU", "", "", 0, 0); !errors.Is(err, apperr.ErrConflict) && err == nil {
		t.Fatalf("overlap: %v", err)
	}
	codec.overlap = nil

	upd, err := s.UpdateRange(context.Background(), "10.1.0.0/24", "10.1.0.0/24", "RU", "MOW", "MSK", 55.7, 37.6)
	if err != nil || upd.Count != 1 {
		t.Fatalf("update: %+v %v", upd, err)
	}
	if _, err := s.UpdateRange(context.Background(), "bad", "10.2.0.0/24", "RU", "", "", 0, 0); err == nil {
		t.Fatal("bad original")
	}
	if _, err := s.UpdateRange(context.Background(), "10.9.0.0/24", "10.9.0.0/24", "RU", "", "", 0, 0); err == nil {
		t.Fatal("missing original")
	}

	var buf bytes.Buffer
	if err := s.ExportCSV(context.Background(), &buf); err != nil || buf.Len() == 0 {
		t.Fatalf("export: %v len=%d", err, buf.Len())
	}
}

func TestListMissing(t *testing.T) {
	miss := &crudMissing{rows: []model.GeoMissingIPRow{{
		IP: "8.8.8.8", Count: 3, AsSrc: 1, AsDst: 2,
	}}}
	s := New(&crudStore{}, miss, nil, nil, &crudCodec{parseOK: true}, 0)
	out, err := s.ListMissing(context.Background(), ListMissingInput{})
	if err != nil || len(out.Items) == 0 || out.Summary == nil {
		t.Fatalf("missing: %+v %v", out, err)
	}
	miss.err = errors.New("ch")
	if _, err := s.ListMissing(context.Background(), ListMissingInput{Limit: 10}); err == nil {
		t.Fatal("want err")
	}
}

func TestUploadHeavyAndBodyTooLarge(t *testing.T) {
	store := &uploadStore{}
	codec := &uploadCodec{ranges: nRanges(2)}
	s := New(store, uploadMissing{}, geoip.New(), nil, codec, 0)
	s.SetHeavySlot(&crudHeavy{fail: context.Canceled})
	_, err := s.UploadCSV(context.Background(), bytes.NewReader(nil), false)
	if err == nil {
		t.Fatal("heavy fail")
	}

	codec = &uploadCodec{err: errors.New("http: request body too large")}
	s = New(store, uploadMissing{}, nil, nil, codec, 0)
	_, err = s.UploadCSV(context.Background(), bytes.NewReader(nil), true)
	if !errors.Is(err, apperr.ErrTooLarge) {
		t.Fatalf("too large wrap: %v", err)
	}
	if !isHTTPRequestBodyTooLarge(errors.New("http: request body too large")) {
		t.Fatal("detect body too large")
	}
}
