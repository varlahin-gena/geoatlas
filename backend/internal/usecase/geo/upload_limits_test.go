package geo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"network_monitor/internal/apperr"
	"network_monitor/internal/geoip"
	"network_monitor/internal/model"
)

type uploadCodec struct {
	ranges []model.GeoRange
	err    error
}

func (c uploadCodec) ReadCSV(r io.Reader) ([]model.GeoRange, error) {
	if c.err != nil {
		return nil, c.err
	}
	_, _ = io.Copy(io.Discard, r)
	out := make([]model.GeoRange, len(c.ranges))
	copy(out, c.ranges)
	return out, nil
}
func (uploadCodec) WriteCSV(io.Writer, []model.GeoRange) error { return nil }
func (uploadCodec) Normalize(ranges []model.GeoRange) ([]model.GeoRange, int) {
	return ranges, 0
}
func (uploadCodec) CheckNonOverlapping([]model.GeoRange) error { return nil }
func (uploadCodec) ParseEntry(string, string, string, string, float64, float64) (model.GeoRange, error) {
	return model.GeoRange{}, nil
}
func (uploadCodec) ParseNetwork(string) (uint32, uint32, bool) { return 0, 0, false }
func (uploadCodec) FormatNetwork(uint32, uint32) string         { return "" }

type uploadStore struct {
	replaced int
}

func (s *uploadStore) Replace(ctx context.Context, ranges []model.GeoRange) (int, error) {
	s.replaced = len(ranges)
	return len(ranges), nil
}
func (*uploadStore) Load(context.Context) ([]model.GeoRange, error) { return nil, nil }
func (*uploadStore) Count(context.Context) (int, error)             { return 0, nil }
func (*uploadStore) FindByIP(context.Context, string) (model.GeoRange, bool, error) {
	return model.GeoRange{}, false, nil
}
func (*uploadStore) ListPage(context.Context, int, string) ([]model.GeoRange, int, int, bool, error) {
	return nil, 0, 0, false, nil
}

type uploadMissing struct{}

func (uploadMissing) ScanGeoMissingIPsForTimeRange(context.Context, model.TimeRange, int, time.Duration) ([]model.GeoMissingIPRow, error) {
	return nil, nil
}

func nRanges(n int) []model.GeoRange {
	out := make([]model.GeoRange, n)
	for i := range out {
		out[i] = model.GeoRange{
			StartIP: uint32(i), EndIP: uint32(i),
			Country: "X", Region: "Y", City: "Z",
		}
	}
	return out
}

func TestUploadCSVRejectsTooManyRanges(t *testing.T) {
	idx := geoip.New()
	store := &uploadStore{}
	svc := New(store, uploadMissing{}, idx, nil, uploadCodec{ranges: nRanges(10)}, 5)

	_, err := svc.UploadCSV(context.Background(), bytes.NewReader(nil), true)
	if !errors.Is(err, apperr.ErrTooLarge) {
		t.Fatalf("err=%v want ErrTooLarge", err)
	}
	if store.replaced != 0 {
		t.Fatal("persist should not run")
	}
}

func TestUploadCSVRejectsDangerousReplace(t *testing.T) {
	idx := geoip.New()
	idx.ReplaceRanges(nRanges(8))
	store := &uploadStore{}
	svc := New(store, uploadMissing{}, idx, nil, uploadCodec{ranges: nRanges(8)}, 10)

	_, err := svc.UploadCSV(context.Background(), bytes.NewReader(nil), false)
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("err=%v want ErrConflict", err)
	}
	if store.replaced != 0 {
		t.Fatal("persist should not run")
	}
}

func TestUploadCSVDryRunAllowsLargeIndex(t *testing.T) {
	idx := geoip.New()
	idx.ReplaceRanges(nRanges(8))
	store := &uploadStore{}
	svc := New(store, uploadMissing{}, idx, nil, uploadCodec{ranges: nRanges(8)}, 10)

	res, err := svc.UploadCSV(context.Background(), bytes.NewReader(nil), true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || res.Count != 8 {
		t.Fatalf("res=%+v", res)
	}
}

func TestUploadCSVSmallReplaceOK(t *testing.T) {
	idx := geoip.New()
	idx.ReplaceRanges(nRanges(2))
	store := &uploadStore{}
	svc := New(store, uploadMissing{}, idx, nil, uploadCodec{ranges: nRanges(3)}, 10)

	res, err := svc.UploadCSV(context.Background(), bytes.NewReader(nil), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 3 || store.replaced != 3 {
		t.Fatalf("res=%+v replaced=%d", res, store.replaced)
	}
}
