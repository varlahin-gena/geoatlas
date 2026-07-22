package geo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"time"

	"network_monitor/internal/model"
	"network_monitor/internal/mapagg"
)

// Service — application use cases для GeoIP.
type Service struct {
	store   RangeStore
	missing MissingIPStore
	index   GeoIndex
	jobs    GeoJobScheduler
	codec   RangeCodec
}

func New(store RangeStore, missing MissingIPStore, index GeoIndex, jobs GeoJobScheduler, codec RangeCodec) *Service {
	return &Service{store: store, missing: missing, index: index, jobs: jobs, codec: codec}
}

// --- Upload ---

type UploadResult struct {
	DryRun   bool
	Count    int
	Sample   []model.GeoRange
	Reload   string
	Backfill string
}

func (s *Service) UploadCSV(ctx context.Context, r io.Reader, dryRun bool) (UploadResult, error) {
	ranges, err := s.codec.ReadCSV(r)
	if err != nil {
		return UploadResult{}, err
	}
	if dryRun {
		sample := ranges
		if len(sample) > 5 {
			sample = sample[:5]
		}
		return UploadResult{DryRun: true, Count: len(ranges), Sample: sample}, nil
	}
	count, err := s.persistRanges(ctx, ranges)
	if err != nil {
		return UploadResult{}, err
	}
	return UploadResult{Count: count, Reload: "applied", Backfill: "scheduled"}, nil
}

// IsClientCSVError — ошибки формата CSV, которые API отдаёт как 400.
func IsClientCSVError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "missing required columns") ||
		strings.Contains(msg, "no valid geo rows") ||
		strings.Contains(msg, "error reading header") ||
		strings.Contains(msg, "overlapping geo ranges")
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
			return ListRangesResult{}, clientErr{fmt.Errorf("invalid IPv4 address")}
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
		return MutateRangeResult{}, clientErr{err}
	}
	existing, err := s.store.Load(ctx)
	if err != nil {
		return MutateRangeResult{}, err
	}
	merged := append(append([]model.GeoRange(nil), existing...), entry)
	if err := s.codec.CheckNonOverlapping(merged); err != nil {
		return MutateRangeResult{}, clientErr{fmt.Errorf("range overlaps existing GeoIP entry: %w", err)}
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
		return MutateRangeResult{}, clientErr{fmt.Errorf("invalid original_network")}
	}
	entry, err := s.codec.ParseEntry(network, country, region, city, lat, lon)
	if err != nil {
		return MutateRangeResult{}, clientErr{err}
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
		return MutateRangeResult{}, notFoundErr{fmt.Errorf("original range not found: %s", strings.TrimSpace(originalNetwork))}
	}
	merged := append(kept, entry)
	if err := s.codec.CheckNonOverlapping(merged); err != nil {
		return MutateRangeResult{}, clientErr{fmt.Errorf("updated range overlaps another GeoIP entry: %w", err)}
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

func (s *Service) FormatNetwork(start, end uint32) string {
	if s.codec == nil {
		return ""
	}
	return s.codec.FormatNetwork(start, end)
}

func (s *Service) persistRanges(ctx context.Context, ranges []model.GeoRange) (int, error) {
	clean, _ := s.codec.Normalize(ranges)
	if len(clean) == 0 {
		return 0, fmt.Errorf("no geo ranges to insert")
	}
	count, err := s.store.Replace(ctx, clean)
	if err != nil {
		return 0, err
	}
	if s.index != nil {
		s.index.ReplaceRanges(clean)
	}
	if s.jobs != nil {
		s.jobs.ScheduleReloadAndEnrich(ctx, 30*time.Minute)
	}
	return count, nil
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

// --- typed errors for HTTP mapping ---

type clientErr struct{ err error }

func (e clientErr) Error() string { return e.err.Error() }
func (e clientErr) Unwrap() error { return e.err }

type notFoundErr struct{ err error }

func (e notFoundErr) Error() string { return e.err.Error() }
func (e notFoundErr) Unwrap() error { return e.err }

func IsClientError(err error) bool {
	var ce clientErr
	return errors.As(err, &ce)
}

func IsNotFound(err error) bool {
	var ne notFoundErr
	return errors.As(err, &ne)
}
