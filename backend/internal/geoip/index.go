package geoip

import (
	"net"
	"sort"
	"strings"
	"sync/atomic"

	"network_monitor/internal/model"
)

// Index — immutable snapshot диапазонов за atomic.Pointer.
// Lookup на hot path без RWMutex; ReplaceRanges публикует новый слайс.
// Загрузка из ClickHouse — через adapter/clickhouse.ReloadableGeoIndex.
type Index struct {
	ranges atomic.Pointer[[]model.GeoRange] // nil или *[]model.GeoRange (не мутировать после Store)
}

func New() *Index {
	return &Index{}
}

func (i *Index) loadRanges() []model.GeoRange {
	p := i.ranges.Load()
	if p == nil {
		return nil
	}
	return *p
}

func (i *Index) storeRanges(ranges []model.GeoRange) {
	// Копия заголовка слайса в куче; сам backing array не шарится с вызывающим.
	cp := ranges
	i.ranges.Store(&cp)
}

// ReplaceRanges заменяет индекс (для тестов и in-memory сценариев).
// Пересечения отбрасываются через NormalizeRanges.
func (i *Index) ReplaceRanges(ranges []model.GeoRange) {
	clean, _ := NormalizeRanges(ranges)
	i.storeRanges(clean)
}

// RangeCount возвращает число загруженных диапазонов.
func (i *Index) RangeCount() int {
	return len(i.loadRanges())
}

// Snapshot возвращает копию текущих диапазонов индекса.
func (i *Index) Snapshot() []model.GeoRange {
	ranges := i.loadRanges()
	if len(ranges) == 0 {
		return nil
	}
	out := make([]model.GeoRange, len(ranges))
	copy(out, ranges)
	return out
}

// LookupRange — O(log n) поиск покрывающего диапазона без копии всего индекса.
func (i *Index) LookupRange(ipStr string) (model.GeoRange, bool) {
	parsed := net.ParseIP(strings.TrimSpace(ipStr))
	v4 := parsed.To4()
	if v4 == nil {
		return model.GeoRange{}, false
	}
	return findContainingRangeLocked(i.loadRanges(), IPv4ToUint32(v4))
}

// CollectRanges отдаёт до limit диапазонов (опционально с текстовым фильтром).
// Индекс уже нормализован — без повторного NormalizeRanges / полной копии.
func (i *Index) CollectRanges(limit int, q string) (items []model.GeoRange, total, filtered int, truncated bool) {
	if limit <= 0 {
		limit = 2000
	}
	q = strings.ToLower(strings.TrimSpace(q))
	ranges := i.loadRanges()
	total = len(ranges)
	if total == 0 {
		return nil, 0, 0, false
	}

	if q == "" {
		n := limit
		if n > total {
			n = total
		}
		items = make([]model.GeoRange, n)
		copy(items, ranges[:n])
		return items, total, total, total > limit
	}

	items = make([]model.GeoRange, 0, min(limit, 256))
	for _, g := range ranges {
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
	ranges := i.loadRanges()

	pos := sort.Search(len(ranges), func(k int) bool {
		return ranges[k].StartIP > ip
	}) - 1

	if pos >= 0 && pos < len(ranges) {
		r := ranges[pos]
		if ip >= r.StartIP && ip <= r.EndIP {
			return model.GeoLookup{
				Lat: r.Lat, Lon: r.Lon,
				City: r.City, Region: r.Region, Country: r.Country,
				Found: true,
			}
		}
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
