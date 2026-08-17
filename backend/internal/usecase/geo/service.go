package geo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sort"
	"strings"
	"time"

	"network_monitor/internal/apperr"
	"network_monitor/internal/geoip"
	"network_monitor/internal/mapagg"
	"network_monitor/internal/model"
)

// Service — application use cases для GeoIP.
type Service struct {
	store     RangeStore
	missing   MissingIPStore
	index     GeoIndex
	jobs      GeoJobScheduler
	codec     RangeCodec
	maxRanges int
}

// New создаёт GeoIP service. maxRanges — лимит строк CSV на upload (0 = без лимита ranges, только HTTP bytes).
func New(store RangeStore, missing MissingIPStore, index GeoIndex, jobs GeoJobScheduler, codec RangeCodec, maxRanges int) *Service {
	return &Service{store: store, missing: missing, index: index, jobs: jobs, codec: codec, maxRanges: maxRanges}
}

// --- Upload ---

type UploadResult struct {
	DryRun   bool
	Count    int
	Sample   []model.GeoRange
	Reload   string
	Backfill string
}

// IndexRangeCount — число диапазонов в in-memory индексе (0 если индекса нет).
func (s *Service) IndexRangeCount() int {
	if s == nil || s.index == nil {
		return 0
	}
	return s.index.RangeCount()
}

// IndexReady — готовность in-memory индекса после стартового Reload.
func (s *Service) IndexReady() bool {
	if s == nil || s.index == nil {
		return true
	}
	return s.index.IndexReady()
}

// PrecheckUpload отклоняет опасный full-replace до чтения тела (когда индекс уже крупный).
// dry_run не блокирует — CSV можно проверить без записи.
func (s *Service) PrecheckUpload(dryRun bool) error {
	return s.rejectIfIndexTooLargeForReplace(dryRun)
}

func (s *Service) UploadCSV(ctx context.Context, r io.Reader, dryRun bool) (UploadResult, error) {
	if err := s.rejectIfIndexTooLargeForReplace(dryRun); err != nil {
		slog.Info("geo upload rejected before parse", "dry_run", dryRun, "index_ranges", s.IndexRangeCount(), "err", err.Error())
		return UploadResult{}, err
	}

	started := time.Now()
	ranges, built, err := s.codec.ReadCSVSnapshot(r)
	parseDur := time.Since(started)
	if err != nil {
		if isHTTPRequestBodyTooLarge(err) {
			return UploadResult{}, apperr.TooLarge(err.Error())
		}
		return UploadResult{}, apperr.InvalidCSV(err)
	}
	slog.Info("geo csv parsed",
		"dry_run", dryRun,
		"ranges", len(ranges),
		"index_ranges", s.IndexRangeCount(),
		"duration", parseDur.Round(time.Millisecond).String(),
	)
	if err := s.checkUploadLimits(len(ranges), dryRun); err != nil {
		slog.Info("geo upload rejected after parse", "dry_run", dryRun, "ranges", len(ranges), "err", err.Error())
		return UploadResult{}, err
	}
	if dryRun {
		sample := ranges
		if len(sample) > 5 {
			sample = sample[:5]
		}
		return UploadResult{DryRun: true, Count: len(ranges), Sample: sample}, nil
	}
	count, err := s.persistRangesWithSnapshot(ctx, ranges, built)
	if err != nil {
		return UploadResult{}, err
	}
	return UploadResult{Count: count, Reload: "applied", Backfill: "scheduled"}, nil
}

// isHTTPRequestBodyTooLarge detects http.MaxBytesError without importing net/http.
// MaxBytesError.Error() is the fixed string "http: request body too large".
func isHTTPRequestBodyTooLarge(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if e.Error() == "http: request body too large" {
			return true
		}
	}
	return false
}

// rejectIfIndexTooLargeForReplace — early 409 до ReadCSV, чтобы не удваивать пик RAM.
func (s *Service) rejectIfIndexTooLargeForReplace(dryRun bool) error {
	if dryRun || s.maxRanges <= 0 || s.index == nil {
		return nil
	}
	existing := s.index.RangeCount()
	if existing >= s.maxRanges/2 {
		return apperr.Conflict(fmt.Sprintf(
			"geo index already large (index=%d, limit=%d); full replace would spike RAM — edit via /geo-ranges, or clear geo_ranges / raise GEOIP_UPLOAD_MAX_RANGES and backend memory",
			existing, s.maxRanges,
		))
	}
	return nil
}

// checkUploadLimits отклоняет слишком большой CSV и опасный replace после parse.
func (s *Service) checkUploadLimits(n int, dryRun bool) error {
	if s.maxRanges > 0 && n > s.maxRanges {
		return apperr.TooLarge(fmt.Sprintf(
			"geo csv has %d ranges, limit is %d (GEOIP_UPLOAD_MAX_RANGES); split the file or raise the limit / backend memory",
			n, s.maxRanges,
		))
	}
	if dryRun || s.maxRanges <= 0 || s.index == nil {
		return nil
	}
	existing := s.index.RangeCount()
	if existing == 0 {
		return nil
	}
	// Пик RAM ≈ existing + parsed upload до ReplaceRanges.
	peak := existing + n
	if peak > s.maxRanges && existing >= s.maxRanges/2 {
		return apperr.Conflict(fmt.Sprintf(
			"geo replace would spike RAM (index=%d, upload=%d, limit=%d); skip re-upload if data is unchanged, or clear geo_ranges first / raise GEOIP_UPLOAD_MAX_RANGES",
			existing, n, s.maxRanges,
		))
	}
	return nil
}

// --- Ranges ---

type ListRangesInput struct {
	Limit int
	Query string
	IP    string
}

type ListRangesResult struct {
	Items     []model.GeoRange
	Total     int
	Filtered  int
	Truncated bool
	IP        string
	IPHit     bool
	IPLookup  bool
}

func (s *Service) ListRanges(ctx context.Context, in ListRangesInput) (ListRangesResult, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 2000
	}
	if limit > 10000 {
		limit = 10000
	}

	if ipQ := strings.TrimSpace(in.IP); ipQ != "" {
		if parsed := net.ParseIP(ipQ); parsed == nil || parsed.To4() == nil {
			return ListRangesResult{}, apperr.InvalidInput("invalid IPv4 address")
		}
		var (
			g     model.GeoRange
			ok    bool
			count int
		)
		if s.index != nil && s.index.RangeCount() > 0 {
			count = s.index.RangeCount()
			g, ok = s.index.LookupRange(ipQ)
		} else {
			var err error
			count, err = s.store.Count(ctx)
			if err != nil {
				return ListRangesResult{}, err
			}
			g, ok, err = s.store.FindByIP(ctx, ipQ)
			if err != nil {
				return ListRangesResult{}, err
			}
		}
		out := ListRangesResult{Total: count, IP: ipQ, IPLookup: true, IPHit: ok}
		if ok {
			out.Items = []model.GeoRange{g}
			out.Filtered = 1
		}
		return out, nil
	}

	var (
		page            []model.GeoRange
		total, filtered int
		truncated       bool
		err             error
	)
	if s.index != nil && s.index.RangeCount() > 0 {
		page, total, filtered, truncated = s.index.CollectRanges(limit, in.Query)
	} else {
		page, total, filtered, truncated, err = s.store.ListPage(ctx, limit, in.Query)
		if err != nil {
			return ListRangesResult{}, err
		}
	}
	return ListRangesResult{
		Items: page, Total: total, Filtered: filtered, Truncated: truncated,
	}, nil
}

type MutateRangeResult struct {
	Count    int
	Entry    model.GeoRange
	Label    string
	Reload   string
	Backfill string
}

func (s *Service) AppendRange(ctx context.Context, network, country, region, city string, lat, lon float64) (MutateRangeResult, error) {
	entry, err := s.codec.ParseEntry(network, country, region, city, lat, lon)
	if err != nil {
		return MutateRangeResult{}, apperr.InvalidInput(err.Error())
	}
	existing, err := s.store.Load(ctx)
	if err != nil {
		return MutateRangeResult{}, err
	}
	merged := append(append([]model.GeoRange(nil), existing...), entry)
	if err := s.codec.CheckNonOverlapping(merged); err != nil {
		return MutateRangeResult{}, apperr.Conflict(fmt.Sprintf("range overlaps existing GeoIP entry: %v", err))
	}
	count, err := s.persistRanges(ctx, merged)
	if err != nil {
		return MutateRangeResult{}, err
	}
	return MutateRangeResult{
		Count: count, Entry: entry,
		Label:  s.codec.FormatNetwork(entry.StartIP, entry.EndIP),
		Reload: "applied", Backfill: "scheduled",
	}, nil
}

func (s *Service) UpdateRange(ctx context.Context, originalNetwork, network, country, region, city string, lat, lon float64) (MutateRangeResult, error) {
	origStart, origEnd, ok := s.codec.ParseNetwork(originalNetwork)
	if !ok {
		return MutateRangeResult{}, apperr.InvalidInput("invalid original_network")
	}
	entry, err := s.codec.ParseEntry(network, country, region, city, lat, lon)
	if err != nil {
		return MutateRangeResult{}, apperr.InvalidInput(err.Error())
	}
	existing, err := s.store.Load(ctx)
	if err != nil {
		return MutateRangeResult{}, err
	}
	found := false
	kept := make([]model.GeoRange, 0, len(existing))
	for _, g := range existing {
		if g.StartIP == origStart && g.EndIP == origEnd {
			found = true
			continue
		}
		kept = append(kept, g)
	}
	if !found {
		return MutateRangeResult{}, apperr.NotFound(fmt.Sprintf("original range not found: %s", strings.TrimSpace(originalNetwork)))
	}
	merged := append(kept, entry)
	if err := s.codec.CheckNonOverlapping(merged); err != nil {
		return MutateRangeResult{}, apperr.Conflict(fmt.Sprintf("updated range overlaps another GeoIP entry: %v", err))
	}
	count, err := s.persistRanges(ctx, merged)
	if err != nil {
		return MutateRangeResult{}, err
	}
	return MutateRangeResult{
		Count: count, Entry: entry,
		Label:  s.codec.FormatNetwork(entry.StartIP, entry.EndIP),
		Reload: "applied", Backfill: "scheduled",
	}, nil
}

func (s *Service) ExportCSV(ctx context.Context, w io.Writer) error {
	ranges, err := s.store.Load(ctx)
	if err != nil {
		return err
	}
	clean, _ := s.codec.Normalize(ranges)
	sort.Slice(clean, func(i, j int) bool {
		if clean[i].StartIP == clean[j].StartIP {
			return clean[i].EndIP < clean[j].EndIP
		}
		return clean[i].StartIP < clean[j].StartIP
	})
	return s.codec.WriteCSV(w, clean)
}

// ClearResult — итог полной очистки geo_ranges + in-memory индекса.
type ClearResult struct {
	IndexBefore int
}

// ClearAll truncate ClickHouse geo_ranges и обнуляет RAM-индекс (без рестарта процесса).
// После этого POST /upload-geo снова принимает полный CSV без early 409.
func (s *Service) ClearAll(ctx context.Context) (ClearResult, error) {
	before := s.IndexRangeCount()
	if s.store == nil {
		return ClearResult{}, fmt.Errorf("geo store unavailable")
	}
	if err := s.store.Truncate(ctx); err != nil {
		return ClearResult{}, err
	}
	if s.index != nil {
		s.index.ReplaceRanges(nil)
	}
	slog.Info("geo ranges cleared",
		"index_before", before,
		"index_after", s.IndexRangeCount(),
		"index_bytes_mb", float64(s.indexApproxBytes())/(1<<20),
	)
	return ClearResult{IndexBefore: before}, nil
}

func (s *Service) FormatNetwork(start, end uint32) string {
	if s.codec == nil {
		return ""
	}
	return s.codec.FormatNetwork(start, end)
}

func (s *Service) persistRanges(ctx context.Context, ranges []model.GeoRange) (int, error) {
	return s.persistRangesWithSnapshot(ctx, ranges, nil)
}

func (s *Service) persistRangesWithSnapshot(ctx context.Context, ranges []model.GeoRange, built *geoip.BuiltSnapshot) (int, error) {
	clean := ranges
	if built == nil {
		clean, _ = s.codec.Normalize(ranges)
	}
	if len(clean) == 0 {
		return 0, fmt.Errorf("no geo ranges to insert")
	}
	count, err := s.store.Replace(ctx, clean)
	if err != nil {
		return 0, err
	}
	if s.index != nil {
		if built != nil {
			s.index.ReplaceBuiltSnapshot(built)
		} else {
			s.index.ReplaceNormalizedRanges(clean)
		}
		slog.Info("geo index updated from persisted ranges",
			"ranges", len(clean),
			"index_bytes_mb", float64(s.indexApproxBytes())/(1<<20),
		)
	}
	if s.jobs != nil {
		s.jobs.ScheduleReloadAndEnrich(ctx, 30*time.Minute)
	}
	return count, nil
}

func (s *Service) indexApproxBytes() uint64 {
	if s == nil || s.index == nil {
		return 0
	}
	return s.index.ApproxBytes()
}

// --- Missing ---

type MissingItem struct {
	IP         string
	Kind       string
	Count      uint64
	AsSrc      uint64
	AsDst      uint64
	SamplePeer string
	LogCountry string
	LogCity    string
	ActionHint string
}

type ListMissingInput struct {
	TimeRange model.TimeRange
	Limit     int
	Timeout   time.Duration
}

type ListMissingResult struct {
	Items   []MissingItem
	Summary map[string]any
	Period  string
	Amount  int
	From    time.Time
	To      time.Time
	Limit   int
}

func (s *Service) ListMissing(ctx context.Context, in ListMissingInput) (ListMissingResult, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	timeout := in.Timeout
	if timeout <= 0 {
		timeout = time.Minute
	}
	rows, err := s.missing.ScanGeoMissingIPsForTimeRange(ctx, in.TimeRange, limit, timeout)
	if err != nil {
		return ListMissingResult{}, err
	}
	var lookuper GeoLookuper
	if s.index != nil {
		lookuper = s.index
	}
	items := filterGeoMissingRows(rows, lookuper)
	return ListMissingResult{
		Items:   items,
		Summary: summarizeGeoMissingItems(items),
		Period:  in.TimeRange.Mode,
		Amount:  in.TimeRange.Amount,
		From:    in.TimeRange.From,
		To:      in.TimeRange.To,
		Limit:   limit,
	}, nil
}

func classifyIPKind(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "invalid"
	}
	if ip.IsLoopback() {
		return "loopback"
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return "link_local"
	}
	if ip.IsMulticast() {
		return "multicast"
	}
	if ip.IsPrivate() || ip.IsUnspecified() {
		return "private"
	}
	return "public_unknown"
}

func actionHintForKind(kind string) string {
	switch kind {
	case "private", "loopback", "link_local", "multicast":
		return "Внутренний/спец. адрес — на карте не отображается; GeoIP обычно не нужен"
	case "invalid":
		return "Некорректный IP — проверьте парсер / исходный лог"
	default:
		return "Публичный IP без GeoIP — добавьте диапазон кнопкой «добавить в базу»"
	}
}

func ipHasMapCoords(geo GeoLookuper, ip string, hint mapagg.LogGeoHint) bool {
	if hint.Lat != 0 || hint.Lon != 0 {
		return true
	}
	if geo == nil {
		return false
	}
	lk := geo.Lookup(ip)
	return lk.Found && (lk.Lat != 0 || lk.Lon != 0)
}

func filterGeoMissingRows(rows []model.GeoMissingIPRow, geo GeoLookuper) []MissingItem {
	items := make([]MissingItem, 0, len(rows))
	for _, row := range rows {
		ip := strings.TrimSpace(row.IP)
		if ip == "" {
			continue
		}
		hint := mapagg.LogGeoHint{
			Lat: row.LogLat, Lon: row.LogLon,
			City: row.LogCity, Country: row.LogCountry,
		}
		if ipHasMapCoords(geo, ip, hint) {
			continue
		}
		kind := classifyIPKind(ip)
		items = append(items, MissingItem{
			IP:         ip,
			Kind:       kind,
			Count:      row.Count,
			AsSrc:      row.AsSrc,
			AsDst:      row.AsDst,
			SamplePeer: row.SamplePeer,
			LogCountry: row.LogCountry,
			LogCity:    row.LogCity,
			ActionHint: actionHintForKind(kind),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].IP < items[j].IP
	})
	return items
}

func summarizeGeoMissingItems(items []MissingItem) map[string]any {
	byKind := map[string]int{}
	var events uint64
	for _, it := range items {
		byKind[it.Kind]++
		events += it.Count
	}
	return map[string]any{
		"unique_ips":   len(items),
		"shown":        len(items),
		"events":       events,
		"by_kind":      byKind,
		"public_focus": byKind["public_unknown"],
	}
}
