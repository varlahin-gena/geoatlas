package geo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"network_monitor/internal/apperr"
	"network_monitor/internal/geoip"
	"network_monitor/internal/model"
)

type uploadCodec struct {
	ranges    []model.GeoRange
	err       error
	readCalls atomic.Int32
}

func (c *uploadCodec) ReadCSV(r io.Reader) ([]model.GeoRange, error) {
	c.readCalls.Add(1)
	if c.err != nil {
		return nil, c.err
	}
	_, _ = io.Copy(io.Discard, r)
	out := make([]model.GeoRange, len(c.ranges))
	copy(out, c.ranges)
	return out, nil
}
func (c *uploadCodec) ReadCSVSnapshot(r io.Reader) ([]model.GeoRange, *geoip.BuiltSnapshot, error) {
	ranges, err := c.ReadCSV(r)
	return ranges, nil, err
}
func (*uploadCodec) WriteCSV(io.Writer, []model.GeoRange) error { return nil }
func (*uploadCodec) Normalize(ranges []model.GeoRange) ([]model.GeoRange, int) {
	return ranges, 0
}
func (*uploadCodec) CheckNonOverlapping([]model.GeoRange) error { return nil }
func (*uploadCodec) ParseEntry(string, string, string, string, float64, float64) (model.GeoRange, error) {
	return model.GeoRange{}, nil
}
func (*uploadCodec) ParseNetwork(string) (uint32, uint32, bool) { return 0, 0, false }
func (*uploadCodec) FormatNetwork(uint32, uint32) string         { return "" }

type uploadStore struct {
	replaced int
}

func (s *uploadStore) Replace(ctx context.Context, ranges []model.GeoRange) (int, error) {
	s.replaced = len(ranges)
	return len(ranges), nil
}
func (s *uploadStore) Truncate(context.Context) error {
	s.replaced = 0
	return nil
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
	codec := &uploadCodec{ranges: nRanges(10)}
	svc := New(store, uploadMissing{}, idx, nil, codec, 5)

	_, err := svc.UploadCSV(context.Background(), bytes.NewReader(nil), true)
	if !errors.Is(err, apperr.ErrTooLarge) {
		t.Fatalf("err=%v want ErrTooLarge", err)
	}
	if store.replaced != 0 {
		t.Fatal("persist should not run")
	}
	if codec.readCalls.Load() != 1 {
		t.Fatalf("dry_run should parse CSV, reads=%d", codec.readCalls.Load())
	}
}

func TestUploadCSVEarlyRejectWithoutReadCSV(t *testing.T) {
	idx := geoip.New()
	idx.ReplaceRanges(nRanges(8)) // >= maxRanges/2
	store := &uploadStore{}
	codec := &uploadCodec{ranges: nRanges(8)}
	svc := New(store, uploadMissing{}, idx, nil, codec, 10)

	_, err := svc.UploadCSV(context.Background(), bytes.NewReader(nil), false)
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("err=%v want ErrConflict", err)
	}
	if codec.readCalls.Load() != 0 {
		t.Fatalf("ReadCSV must not run on early reject, reads=%d", codec.readCalls.Load())
	}
	if store.replaced != 0 {
		t.Fatal("persist should not run")
	}
}

func TestUploadCSVDryRunAllowsLargeIndex(t *testing.T) {
	idx := geoip.New()
	idx.ReplaceRanges(nRanges(8))
	store := &uploadStore{}
	codec := &uploadCodec{ranges: nRanges(8)}
	svc := New(store, uploadMissing{}, idx, nil, codec, 10)

	res, err := svc.UploadCSV(context.Background(), bytes.NewReader(nil), true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.DryRun || res.Count != 8 {
		t.Fatalf("res=%+v", res)
	}
	if codec.readCalls.Load() != 1 {
		t.Fatalf("dry_run should parse, reads=%d", codec.readCalls.Load())
	}
}

func TestUploadCSVSmallReplaceOK(t *testing.T) {
	idx := geoip.New()
	idx.ReplaceRanges(nRanges(2)) // < maxRanges/2
	store := &uploadStore{}
	codec := &uploadCodec{ranges: nRanges(3)}
	svc := New(store, uploadMissing{}, idx, nil, codec, 10)

	res, err := svc.UploadCSV(context.Background(), bytes.NewReader(nil), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 3 || store.replaced != 3 {
		t.Fatalf("res=%+v replaced=%d", res, store.replaced)
	}
	if codec.readCalls.Load() != 1 {
		t.Fatalf("reads=%d", codec.readCalls.Load())
	}
}

func TestClearAllResetsIndex(t *testing.T) {
	idx := geoip.New()
	idx.ReplaceRanges(nRanges(8))
	store := &uploadStore{}
	svc := New(store, uploadMissing{}, idx, nil, &uploadCodec{}, 10)

	res, err := svc.ClearAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.IndexBefore != 8 {
		t.Fatalf("IndexBefore=%d", res.IndexBefore)
	}
	if idx.RangeCount() != 0 {
		t.Fatalf("index still %d", idx.RangeCount())
	}
	if err := svc.PrecheckUpload(false); err != nil {
		t.Fatalf("after clear upload should be allowed: %v", err)
	}
}

func TestPrecheckUpload(t *testing.T) {
	idx := geoip.New()
	idx.ReplaceRanges(nRanges(8))
	svc := New(&uploadStore{}, uploadMissing{}, idx, nil, &uploadCodec{}, 10)
	if err := svc.PrecheckUpload(false); !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("err=%v", err)
	}
	if err := svc.PrecheckUpload(true); err != nil {
		t.Fatalf("dry_run precheck: %v", err)
	}
}

func TestUploadCSVRejectsMemHeadroom(t *testing.T) {
	idx := geoip.New()
	codec := &uploadCodec{ranges: nRanges(3)}
	svc := New(&uploadStore{}, uploadMissing{}, idx, nil, codec, 0)
	svc.SetSoftMemLimitBytes(1) // 1 byte — любой commit не пройдёт

	_, err := svc.UploadCSV(context.Background(), bytes.NewReader(nil), false)
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("err=%v want ErrConflict", err)
	}
	_, err = svc.UploadCSV(context.Background(), bytes.NewReader(nil), true)
	if err != nil {
		t.Fatalf("dry_run must skip headroom: %v", err)
	}
}
