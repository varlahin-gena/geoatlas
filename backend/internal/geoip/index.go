package geoip

import (
	"net"
	"sort"
	"strings"
	"sync/atomic"
	"unsafe"

	"network_monitor/internal/model"
)

type rangeRow struct {
	StartIP   uint32
	EndIP     uint32
	CountryID uint32
	RegionID  uint32
	CityID    uint32
	Lat       float64
	Lon       float64
}

var (
	rangeRowSize    = uint64(unsafe.Sizeof(rangeRow{}))
	stringHeaderSize = uint64(unsafe.Sizeof(""))
)

type snapshot struct {
	rows        []rangeRow
	countries   []string
	regions     []string
	cities      []string
	approxBytes uint64
}

type dictionaryBuilder struct {
	ids    map[string]uint32
	values []string
	bytes  uint64
}

func newDictionaryBuilder(capHint int) dictionaryBuilder {
	if capHint <= 0 {
		capHint = 1
	}
	return dictionaryBuilder{
		ids:    make(map[string]uint32, capHint),
		values: make([]string, 0, capHint),
	}
}

func (b *dictionaryBuilder) ID(value string) uint32 {
	if id, ok := b.ids[value]; ok {
		return id
	}
	id := uint32(len(b.values))
	b.ids[value] = id
	b.values = append(b.values, value)
	b.bytes += uint64(len(value))
	return id
}

func buildSnapshot(ranges []model.GeoRange, alreadyNormalized bool) (*snapshot, int) {
	if len(ranges) == 0 {
		return &snapshot{}, 0
	}
	clean := ranges
	skipped := 0
	if !alreadyNormalized {
		clean, skipped = NormalizeRanges(ranges)
	}
	if len(clean) == 0 {
		return &snapshot{}, skipped
	}
	countries := newDictionaryBuilder(min(len(clean), 256))
	regions := newDictionaryBuilder(min(len(clean), 512))
	cities := newDictionaryBuilder(min(len(clean), 2048))
	rows := make([]rangeRow, 0, len(clean))
	for _, g := range clean {
		rows = append(rows, rangeRow{
			StartIP:   g.StartIP,
			EndIP:     g.EndIP,
			CountryID: countries.ID(g.Country),
			RegionID:  regions.ID(g.Region),
			CityID:    cities.ID(g.City),
			Lat:       g.Lat,
			Lon:       g.Lon,
		})
	}
	return &snapshot{
		rows:      rows,
		countries: countries.values,
		regions:   regions.values,
		cities:    cities.values,
		approxBytes: uint64(cap(rows))*rangeRowSize +
			uint64(cap(countries.values))*stringHeaderSize + countries.bytes +
			uint64(cap(regions.values))*stringHeaderSize + regions.bytes +
			uint64(cap(cities.values))*stringHeaderSize + cities.bytes,
	}, skipped
}

func (s *snapshot) lookupString(dict []string, id uint32) string {
	if int(id) < len(dict) {
		return dict[id]
	}
	return ""
}

func (s *snapshot) toGeoRange(row rangeRow) model.GeoRange {
	return model.GeoRange{
		StartIP: row.StartIP,
		EndIP:   row.EndIP,
		Country: s.lookupString(s.countries, row.CountryID),
		Region:  s.lookupString(s.regions, row.RegionID),
		City:    s.lookupString(s.cities, row.CityID),
		Lat:     row.Lat,
		Lon:     row.Lon,
	}
}

func (s *snapshot) toGeoLookup(row rangeRow) model.GeoLookup {
	return model.GeoLookup{
		Lat:     row.Lat,
		Lon:     row.Lon,
		City:    s.lookupString(s.cities, row.CityID),
		Region:  s.lookupString(s.regions, row.RegionID),
		Country: s.lookupString(s.countries, row.CountryID),
		Found:   true,
	}
}

func findContainingRow(rows []rangeRow, ip uint32) (rangeRow, bool) {
	if len(rows) == 0 {
		return rangeRow{}, false
	}
	pos := sort.Search(len(rows), func(k int) bool {
		return rows[k].StartIP > ip
	}) - 1
	if pos >= 0 && pos < len(rows) {
		r := rows[pos]
		if ip >= r.StartIP && ip <= r.EndIP {
			return r, true
		}
	}
	return rangeRow{}, false
}

// Index — immutable snapshot диапазонов за atomic.Pointer.
// Lookup на hot path без RWMutex; ReplaceRanges публикует новый compact snapshot.
// Загрузка из ClickHouse — через adapter/clickhouse/geostore.ReloadableGeoIndex.
type Index struct {
	data atomic.Pointer[snapshot] // nil или *snapshot (не мутировать после Store)
}

func New() *Index {
	return &Index{}
}

func (i *Index) loadSnapshot() *snapshot {
	p := i.data.Load()
	if p == nil {
		return nil
	}
	return p
}

func (i *Index) storeSnapshot(snap *snapshot) {
	if snap == nil {
		snap = &snapshot{}
	}
	i.data.Store(snap)
}

// Built возвращает текущий compact snapshot для persist на диск.
func (i *Index) Built() *BuiltSnapshot {
	if i == nil {
		return &BuiltSnapshot{snap: &snapshot{}}
	}
	snap := i.loadSnapshot()
	if snap == nil {
		return &BuiltSnapshot{snap: &snapshot{}}
	}
	return &BuiltSnapshot{snap: snap}
}

// ReplaceRanges заменяет индекс (для тестов и in-memory сценариев).
// Пересечения отбрасываются через NormalizeRanges.
func (i *Index) ReplaceRanges(ranges []model.GeoRange) {
	snap, _ := buildSnapshot(ranges, false)
	i.storeSnapshot(snap)
}

// ReplaceNormalizedRanges публикует уже нормализованные диапазоны без повторного NormalizeRanges.
func (i *Index) ReplaceNormalizedRanges(ranges []model.GeoRange) {
	snap, _ := buildSnapshot(ranges, true)
	i.storeSnapshot(snap)
}

// ReplaceBuiltSnapshot публикует заранее собранный compact snapshot.
func (i *Index) ReplaceBuiltSnapshot(built *BuiltSnapshot) {
	if built == nil {
		i.storeSnapshot(nil)
		return
	}
	i.storeSnapshot(built.snap)
}

// RangeCount возвращает число загруженных диапазонов.
func (i *Index) RangeCount() int {
	snap := i.loadSnapshot()
	if snap == nil {
		return 0
	}
	return len(snap.rows)
}

// IndexReady — in-memory index всегда готов к lookup после создания.
func (i *Index) IndexReady() bool {
	return i != nil
}

// ApproxBytes возвращает оценку RAM текущего compact snapshot.
func (i *Index) ApproxBytes() uint64 {
	snap := i.loadSnapshot()
	if snap == nil {
		return 0
	}
	return snap.approxBytes
}

// Snapshot возвращает копию текущих диапазонов индекса.
func (i *Index) Snapshot() []model.GeoRange {
	snap := i.loadSnapshot()
	if snap == nil || len(snap.rows) == 0 {
		return nil
	}
	out := make([]model.GeoRange, len(snap.rows))
	for idx, row := range snap.rows {
		out[idx] = snap.toGeoRange(row)
	}
	return out
}

// LookupRange — O(log n) поиск покрывающего диапазона без копии всего индекса.
func (i *Index) LookupRange(ipStr string) (model.GeoRange, bool) {
	parsed := net.ParseIP(strings.TrimSpace(ipStr))
	v4 := parsed.To4()
	if v4 == nil {
		return model.GeoRange{}, false
	}
	snap := i.loadSnapshot()
	if snap == nil {
		return model.GeoRange{}, false
	}
	row, ok := findContainingRow(snap.rows, IPv4ToUint32(v4))
	if !ok {
		return model.GeoRange{}, false
	}
	return snap.toGeoRange(row), true
}

// CollectRanges отдаёт до limit диапазонов (опционально с текстовым фильтром).
// Индекс уже нормализован — без повторного NormalizeRanges / полной копии.
func (i *Index) CollectRanges(limit int, q string) (items []model.GeoRange, total, filtered int, truncated bool) {
	if limit <= 0 {
		limit = 2000
	}
	if limit > 10000 {
		limit = 10000
	}
	q = strings.ToLower(strings.TrimSpace(q))
	snap := i.loadSnapshot()
	if snap == nil {
		return nil, 0, 0, false
	}
	total = len(snap.rows)
	if total == 0 {
		return nil, 0, 0, false
	}

	if q == "" {
		n := limit
		if n > total {
			n = total
		}
		items = make([]model.GeoRange, n)
		for idx := range n {
			items[idx] = snap.toGeoRange(snap.rows[idx])
		}
		return items, total, total, total > limit
	}

	items = make([]model.GeoRange, 0, min(limit, 256))
	for _, row := range snap.rows {
		g := snap.toGeoRange(row)
		if !rangeMatchesQuery(g, q) {
			continue
		}
		filtered++
		if len(items) < limit {
			items = append(items, g)
		}
	}
	return items, total, filtered, filtered > limit
}

func rangeMatchesQuery(g model.GeoRange, qLower string) bool {
	if strings.Contains(strings.ToLower(g.Country), qLower) ||
		strings.Contains(strings.ToLower(g.Region), qLower) ||
		strings.Contains(strings.ToLower(g.City), qLower) {
		return true
	}
	return strings.Contains(strings.ToLower(FormatNetwork(g.StartIP, g.EndIP)), qLower)
}

func findContainingRangeLocked(ranges []model.GeoRange, ip uint32) (model.GeoRange, bool) {
	if len(ranges) == 0 {
		return model.GeoRange{}, false
	}
	pos := sort.Search(len(ranges), func(k int) bool {
		return ranges[k].StartIP > ip
	}) - 1
	if pos >= 0 && pos < len(ranges) {
		r := ranges[pos]
		if ip >= r.StartIP && ip <= r.EndIP {
			return r, true
		}
	}
	return model.GeoRange{}, false
}

func (i *Index) Lookup(ipStr string) model.GeoLookup {
	if i == nil {
		return model.GeoLookup{Country: "Неизвестно"}
	}
	parsed := net.ParseIP(strings.TrimSpace(ipStr))
	v4 := parsed.To4()
	if v4 == nil {
		return model.GeoLookup{Country: "Неизвестно"}
	}
	ip := IPv4ToUint32(v4)
	snap := i.loadSnapshot()
	if snap == nil {
		return model.GeoLookup{Country: "Неизвестно"}
	}
	row, ok := findContainingRow(snap.rows, ip)
	if ok {
		return snap.toGeoLookup(row)
	}
	return model.GeoLookup{Country: "Неизвестно"}
}

// IPv4ToUint32 кодирует net.IP (уже IPv4) в uint32 без повторного ParseIP.
func IPv4ToUint32(ip net.IP) uint32 {
	ip4 := ip.To4()
	if ip4 == nil {
		return 0
	}
	return uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
}

func IPToUint32(ipStr string) uint32 {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" {
		return 0
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return 0
	}
	return IPv4ToUint32(ip)
}

// Uint32ToIP форматирует IPv4 из UInt32 в dotted-quad.
func Uint32ToIP(v uint32) string {
	return net.IPv4(byte(v>>24), byte(v>>16), byte(v>>8), byte(v)).String()
}

// FindContainingRange ищет диапазон, покрывающий IPv4. ok=false если IP невалиден или не найден.
func FindContainingRange(ranges []model.GeoRange, ipStr string) (model.GeoRange, bool) {
	parsed := net.ParseIP(strings.TrimSpace(ipStr))
	v4 := parsed.To4()
	if v4 == nil {
		return model.GeoRange{}, false
	}
	return findContainingRangeLocked(ranges, IPv4ToUint32(v4))
}
