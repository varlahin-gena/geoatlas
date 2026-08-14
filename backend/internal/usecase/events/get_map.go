package events

import (
	"context"
	"reflect"
	"strings"
	"time"

	"network_monitor/internal/mapagg"
	"network_monitor/internal/model"
)

// GetMapInput — параметры построения карты.
type GetMapInput struct {
	TimeRange model.TimeRange
	Limit     int
	GroupBy   string // ip|subnet|city|country
	Filter    string // all|allowed|blocked
	Country   string
	Query     string
	Timeout   time.Duration
}

// MapScanQuery — параметры скана агрегатов карты (LIMIT после action/country/q).
type MapScanQuery struct {
	GroupBy string
	Limit   int
	Filter  string
	Country string
	Query   string
}

// GetMapResult — результат для HTTP/API слоя.
type GetMapResult struct {
	Lines        []model.Line
	Points       map[string]model.Node
	RawPairs     int
	SkippedNoGeo int
	Source       string
	GroupBy      string
	Filter       string
	Country      string
	Query        string
	Period       string
	Amount       int
	From         time.Time
	To           time.Time
}

// Service — application use cases для карты/events.
type Service struct {
	traffic    TrafficRepository
	geo        GeoLookuper
	reputation ReputationLookuper
}

func New(traffic TrafficRepository, geo GeoLookuper, reputation ReputationLookuper) *Service {
	return &Service{traffic: traffic, geo: geo, reputation: reputation}
}

// GetMap строит линии/узлы: сначала pre-agg geo edges, иначе live IP + GeoIP.
func (s *Service) GetMap(ctx context.Context, in GetMapInput) (GetMapResult, error) {
	groupBy := normalizeGroupBy(in.GroupBy)
	filter := normalizeFilter(in.Filter)
	country := clipRunes(in.Country, 80)
	queryText := clipRunes(in.Query, 120)
	out := GetMapResult{
		GroupBy: groupBy,
		Filter:  filter,
		Country: country,
		Query:   queryText,
		Period:  in.TimeRange.Mode,
		Amount:  in.TimeRange.Amount,
		From:    in.TimeRange.From,
		To:      in.TimeRange.To,
		Points:  map[string]model.Node{},
		Source:  "ip",
	}

	scan, err := s.traffic.ScanMapAggs(ctx, in.TimeRange, MapScanQuery{
		GroupBy: groupBy,
		Limit:   in.Limit,
		Filter:  filter,
		Country: country,
		Query:   queryText,
	}, in.Timeout)
	if err != nil {
		return GetMapResult{}, err
	}
	if len(scan.GeoEdges) > 0 {
		lines, points, skipped := mapagg.BuildMapFromGeoEdges(scan.GeoEdges)
		out.Lines = lines
		out.Points = points
		out.SkippedNoGeo = skipped
		out.RawPairs = len(scan.GeoEdges)
		out.Source = scan.Source
	} else if len(scan.Raws) > 0 {
		out.RawPairs = len(scan.Raws)
		out.Lines, out.Points, out.SkippedNoGeo = buildMapFromIPRaws(scan.Raws, s.geo, groupBy)
		out.Source = scan.Source
	}

	if out.Points == nil {
		out.Points = map[string]model.Node{}
	}
	if groupBy == "ip" {
		enrichMapReputation(out.Lines, out.Points, s.reputation)
	}
	return out, nil
}

// reputationLookuperNil — true для nil interface и typed-nil (*T)(nil) в interface.
// Обычный `rep == nil` typed-nil не ловит → Lookup паникует на promoted-методах.
func reputationLookuperNil(rep ReputationLookuper) bool {
	if rep == nil {
		return true
	}
	v := reflect.ValueOf(rep)
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return v.IsNil()
	default:
		return false
	}
}

func enrichMapReputation(lines []model.Line, points map[string]model.Node, rep ReputationLookuper) {
	if reputationLookuperNil(rep) {
		return
	}
	cache := map[string][]model.ReputationHit{}
	lookup := func(ip string) []model.ReputationHit {
		if ip == "" {
			return nil
		}
		if h, ok := cache[ip]; ok {
			return h
		}
		h := rep.Lookup(ip)
		cache[ip] = h
		return h
	}
	for i := range lines {
		lines[i].SrcReputation = lookup(lines[i].Src)
		lines[i].DstReputation = lookup(lines[i].Dst)
	}
	for k, n := range points {
		n.Reputation = lookup(k)
		points[k] = n
	}
}

func normalizeGroupBy(v string) string {
	switch v {
	case "ip", "subnet", "country", "city":
		return v
	default:
		return "city"
	}
}

func normalizeFilter(v string) string {
	switch v {
	case "allowed", "blocked":
		return v
	default:
		return "all"
	}
}

func clipRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || max < 1 {
		return ""
	}
	s = strings.ReplaceAll(s, "\x00", "")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func buildMapFromIPRaws(raws []model.RawAgg, geo mapagg.GeoLookuper, groupBy string) ([]model.Line, map[string]model.Node, int) {
	edgesMap := make(map[string]*mapagg.EdgeAgg)
	srcCache := make(map[string]model.GroupMeta)
	dstCache := make(map[string]model.GroupMeta)

	metaFor := func(ip string, hint mapagg.LogGeoHint, cache map[string]model.GroupMeta) model.GroupMeta {
		if m, ok := cache[ip]; ok {
			if m.Valid {
				return m
			}
			m2 := mapagg.IPGroupMetaHinted(geo, ip, groupBy, hint)
			if m2.Valid {
				cache[ip] = m2
				return m2
			}
			return m
		}
		m := mapagg.IPGroupMetaHinted(geo, ip, groupBy, hint)
		cache[ip] = m
		return m
	}

	for _, row := range raws {
		s := metaFor(row.SrcIP, mapagg.LogGeoHint{
			Lat: row.SrcLat, Lon: row.SrcLon, City: row.SrcCity, Country: row.SrcCountry,
		}, srcCache)
		d := metaFor(row.DstIP, mapagg.LogGeoHint{
			Lat: row.DstLat, Lon: row.DstLon, City: row.DstCity, Country: row.DstCountry,
		}, dstCache)
		if !s.Valid || !d.Valid {
			continue
		}
		key := s.Key + "→" + d.Key
		edge, ok := edgesMap[key]
		if !ok {
			edge = &mapagg.EdgeAgg{}
			edgesMap[key] = edge
		}
		edge.Add(row, s, d)
	}

	lines := make([]model.Line, 0, len(edgesMap))
	active := make(map[string]struct{})
	skippedNoGeo := 0
	for _, e := range edgesMap {
		if e.CoordWeight == 0 {
			skippedNoGeo++
			continue
		}
		for _, ln := range e.ToLines() {
			lines = append(lines, ln)
			active[ln.Src] = struct{}{}
			active[ln.Dst] = struct{}{}
		}
	}

	nodeMap := make(map[string]*mapagg.NodeAgg)
	for _, row := range raws {
		s := metaFor(row.SrcIP, mapagg.LogGeoHint{
			Lat: row.SrcLat, Lon: row.SrcLon, City: row.SrcCity, Country: row.SrcCountry,
		}, srcCache)
		d := metaFor(row.DstIP, mapagg.LogGeoHint{
			Lat: row.DstLat, Lon: row.DstLon, City: row.DstCity, Country: row.DstCountry,
		}, dstCache)

		if _, ok := active[s.Key]; ok {
			n, ex := nodeMap[s.Key]
			if !ex {
				n = &mapagg.NodeAgg{}
				nodeMap[s.Key] = n
			}
			n.Add(s, row.Count)
		}
		if _, ok := active[d.Key]; ok {
			n, ex := nodeMap[d.Key]
			if !ex {
				n = &mapagg.NodeAgg{}
				nodeMap[d.Key] = n
			}
			n.Add(d, row.Count)
		}
	}

	points := make(map[string]model.Node, len(nodeMap))
	for k, n := range nodeMap {
		points[k] = n.ToNode()
	}
	return lines, points, skippedNoGeo
}
