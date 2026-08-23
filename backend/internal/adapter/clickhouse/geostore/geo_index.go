package geostore

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ClickHouse/clickhouse-go/v2"

	"geoatlas/internal/fileatomic"
	"geoatlas/internal/geoip"
	"geoatlas/internal/model"
)

// ReloadableGeoIndex — *geoip.Index + Reload из ClickHouse + disk snapshot.
type ReloadableGeoIndex struct {
	*geoip.Index
	ch       clickhouse.Conn
	snapPath string
	ready    atomic.Bool

	mu      sync.Mutex
	stamp   geoip.SourceStamp
	okStamp bool
}

func NewReloadableGeoIndex(ch clickhouse.Conn, snapshotPath string) *ReloadableGeoIndex {
	return &ReloadableGeoIndex{
		Index:    geoip.New(),
		ch:       ch,
		snapPath: strings.TrimSpace(snapshotPath),
	}
}

// IndexReady — true после LoadDisk или Reload (успех или пустая база).
func (i *ReloadableGeoIndex) IndexReady() bool {
	return i != nil && i.ready.Load()
}

// LoadDisk поднимает compact snapshot с диска, если файл есть.
// Карта работает до полного Reload из ClickHouse.
func (i *ReloadableGeoIndex) LoadDisk() bool {
	if i == nil || i.Index == nil || i.snapPath == "" {
		return false
	}
	built, stamp, err := readSnapshotFile(i.snapPath)
	if err != nil {
		return false
	}
	i.Index.ReplaceBuiltSnapshot(built)
	i.setStamp(stamp)
	i.ready.Store(true)
	slog.Info("geo index loaded from disk snapshot",
		"ranges", built.RangeCount(),
		"index_bytes_mb", float64(i.ApproxBytes())/(1<<20),
		"stamp_count", stamp.Count,
		"path", i.snapPath,
	)
	return true
}

func (i *ReloadableGeoIndex) Reload(ctx context.Context) error {
	if i == nil || i.Index == nil {
		return nil
	}
	defer i.ready.Store(true)

	before := i.RangeCount()
	var msBefore runtime.MemStats
	runtime.ReadMemStats(&msBefore)

	chStamp, stampErr := QuerySourceStamp(ctx, i.ch)
	if stampErr == nil {
		i.mu.Lock()
		same := i.okStamp && i.stamp.Equal(chStamp)
		i.mu.Unlock()
		if same {
			slog.Info("geo index snapshot matches clickhouse", "ranges", i.RangeCount(), "stamp_count", chStamp.Count)
			return nil
		}
		if built, fileStamp, err := readSnapshotFile(i.snapPath); err == nil && fileStamp.Equal(chStamp) {
			i.Index.ReplaceBuiltSnapshot(built)
			i.setStamp(fileStamp)
			slog.Info("geo index loaded from disk snapshot (clickhouse stamp match)",
				"ranges", built.RangeCount(),
				"stamp_count", chStamp.Count,
			)
			return nil
		}
	} else {
		slog.Warn("geo index: source stamp query failed, full reload", "err", stampErr)
	}

	built, err := LoadGeoSnapshot(ctx, i.ch)
	if err != nil {
		return err
	}
	skipped := 0
	if built != nil {
		skipped = built.Skipped()
	}
	if skipped > 0 {
		slog.Warn("geo index: overlapping or invalid ranges skipped", "skipped", skipped, "kept", built.RangeCount())
	}
	i.Index.ReplaceBuiltSnapshot(built)
	if stampErr == nil {
		i.persist(chStamp)
	} else if st, qerr := QuerySourceStamp(ctx, i.ch); qerr == nil {
		i.persist(st)
	}

	var msAfter runtime.MemStats
	runtime.ReadMemStats(&msAfter)
	slog.Info("geo index loaded",
		"ranges", built.RangeCount(),
		"prev_ranges", before,
		"index_bytes_mb", float64(i.ApproxBytes())/(1<<20),
		"heap_alloc_mb", msAfter.Alloc/(1<<20),
		"heap_sys_mb", msAfter.Sys/(1<<20),
		"heap_delta_mb", int64(msAfter.Alloc-msBefore.Alloc)/(1<<20),
	)
	return nil
}

// ReplaceBuiltSnapshot публикует snapshot и пишет disk cache (CSV upload path).
func (i *ReloadableGeoIndex) ReplaceBuiltSnapshot(built *geoip.BuiltSnapshot) {
	if i == nil || i.Index == nil {
		return
	}
	i.Index.ReplaceBuiltSnapshot(built)
	i.ready.Store(true)
	i.persist(stampFromBuilt(built))
}

// ReplaceNormalizedRanges — persist path без BuiltSnapshot.
func (i *ReloadableGeoIndex) ReplaceNormalizedRanges(ranges []model.GeoRange) {
	if i == nil || i.Index == nil {
		return
	}
	i.Index.ReplaceNormalizedRanges(ranges)
	i.ready.Store(true)
	i.persist(geoip.StampFromRanges(ranges))
}

// ReplaceRanges — ClearAll / тесты: сброс индекса и snapshot-файла.
func (i *ReloadableGeoIndex) ReplaceRanges(ranges []model.GeoRange) {
	if i == nil || i.Index == nil {
		return
	}
	i.Index.ReplaceRanges(ranges)
	i.ready.Store(true)
	i.persist(geoip.StampFromRanges(ranges))
}

func (i *ReloadableGeoIndex) setStamp(stamp geoip.SourceStamp) {
	i.mu.Lock()
	i.stamp = stamp
	i.okStamp = true
	i.mu.Unlock()
}

func (i *ReloadableGeoIndex) persist(stamp geoip.SourceStamp) {
	if i == nil || i.snapPath == "" {
		return
	}
	i.setStamp(stamp)
	if stamp.Count == 0 {
		if err := os.Remove(i.snapPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("geo index snapshot remove failed", "err", err, "path", i.snapPath)
		}
		return
	}
	raw, err := geoip.EncodeSnapshot(i.Built(), stamp)
	if err != nil {
		slog.Warn("geo index snapshot encode failed", "err", err)
		return
	}
	if err := fileatomic.WriteFile(i.snapPath, raw, 0o600); err != nil {
		slog.Warn("geo index snapshot write failed", "err", err, "path", i.snapPath)
	}
}

func readSnapshotFile(path string) (*geoip.BuiltSnapshot, geoip.SourceStamp, error) {
	var zero geoip.SourceStamp
	if path == "" {
		return nil, zero, errors.New("geo snapshot path empty")
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: path from GEOIP_SNAPSHOT_FILE / data dir
	if err != nil {
		return nil, zero, err
	}
	return geoip.DecodeSnapshot(data)
}

func stampFromBuilt(built *geoip.BuiltSnapshot) geoip.SourceStamp {
	if built == nil {
		return geoip.SourceStamp{}
	}
	return geoip.StampFromRanges(built.Ranges())
}
