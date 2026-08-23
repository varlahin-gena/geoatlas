package reputation

import (
	"net"
	"sort"
	"strings"
	"sync/atomic"

	"geoatlas/internal/geoip"
	"geoatlas/internal/model"
	"geoatlas/internal/netutil"
)

// Index — immutable snapshot диапазонов; Lookup допускает пересечения списков.
type Index struct {
	ranges atomic.Pointer[[]model.ReputationRange]
}

func New() *Index {
	return &Index{}
}

func (i *Index) load() []model.ReputationRange {
	p := i.ranges.Load()
	if p == nil {
		return nil
	}
	return *p
}

func (i *Index) store(ranges []model.ReputationRange) {
	cp := ranges
	i.ranges.Store(&cp)
}

// ReplaceAll заменяет весь индекс (уже нормализованный или нет).
func (i *Index) ReplaceAll(ranges []model.ReputationRange) {
	i.store(NormalizeRanges(ranges))
}

// ReplaceList заменяет диапазоны одного list_name, остальные сохраняет.
func (i *Index) ReplaceList(listName string, ranges []model.ReputationRange) {
	listName = strings.TrimSpace(listName)
	cur := i.load()
	kept := make([]model.ReputationRange, 0, len(cur))
	for _, r := range cur {
		if r.ListName != listName {
			kept = append(kept, r)
		}
	}
	for _, r := range ranges {
		r.ListName = listName
		kept = append(kept, r)
	}
	i.store(NormalizeRanges(kept))
}

// DeleteList убирает list_name из индекса.
func (i *Index) DeleteList(listName string) {
	listName = strings.TrimSpace(listName)
	cur := i.load()
	kept := make([]model.ReputationRange, 0, len(cur))
	for _, r := range cur {
		if r.ListName != listName {
			kept = append(kept, r)
		}
	}
	i.store(kept)
}

func (i *Index) RangeCount() int {
	return len(i.load())
}

func (i *Index) Snapshot() []model.ReputationRange {
	ranges := i.load()
	if len(ranges) == 0 {
		return nil
	}
	out := make([]model.ReputationRange, len(ranges))
	copy(out, ranges)
	return out
}

// Lookup возвращает все хиты (разные list/category) для публичного IPv4.
// Частные/спец. адреса (RFC1918, loopback, CGNAT и т.п.) всегда без хитов.
// Nil-receiver безопасен (typed-nil в interface / выключенный модуль).
func (i *Index) Lookup(ipStr string) []model.ReputationHit {
	if i == nil {
		return nil
	}
	parsed := net.ParseIP(strings.TrimSpace(ipStr))
	v4 := parsed.To4()
	if v4 == nil {
		return nil
	}
	if netutil.IsNonPublicIPv4IP(v4) {
		return nil
	}
	ip := geoip.IPv4ToUint32(v4)
	ranges := i.load()
	if len(ranges) == 0 {
		return nil
	}

	// Кандидаты: все с StartIP <= ip (бинарный поиск правой границы).
	pos := sort.Search(len(ranges), func(k int) bool {
		return ranges[k].StartIP > ip
	})
	seen := map[string]model.ReputationHit{}
	for k := 0; k < pos; k++ {
		r := ranges[k]
		if ip > r.EndIP {
			continue
		}
		key := r.ListName + "\x00" + r.Category
		netStr := FormatNetworkPreferCIDR(r.StartIP, r.EndIP)
		prev, ok := seen[key]
		if !ok {
			seen[key] = model.ReputationHit{
				List: r.ListName, Category: r.Category, Network: netStr,
			}
			continue
		}
		// Более узкий диапазон информативнее для UI.
		span := r.EndIP - r.StartIP
		prevStart, prevEnd, okPrev := ParseNetworkField(prev.Network)
		if !okPrev {
			seen[key] = model.ReputationHit{List: r.ListName, Category: r.Category, Network: netStr}
			continue
		}
		prevSpan := prevEnd - prevStart
		if span < prevSpan {
			seen[key] = model.ReputationHit{List: r.ListName, Category: r.Category, Network: netStr}
		}
	}
	hits := make([]model.ReputationHit, 0, len(seen))
	for _, h := range seen {
		hits = append(hits, h)
	}
	sort.Slice(hits, func(a, b int) bool {
		if hits[a].Category != hits[b].Category {
			return hits[a].Category < hits[b].Category
		}
		return hits[a].List < hits[b].List
	})
	return hits
}

// ListMeta агрегирует метаданные по list_name из текущего снимка.
func (i *Index) ListMeta() []model.ReputationListMeta {
	ranges := i.load()
	type agg struct {
		meta model.ReputationListMeta
	}
	by := map[string]*agg{}
	for _, r := range ranges {
		a := by[r.ListName]
		if a == nil {
			a = &agg{meta: model.ReputationListMeta{
				Name:     r.ListName,
				Category: r.Category,
				Source:   r.Source,
			}}
			by[r.ListName] = a
		}
		a.meta.Count++
		if r.UpdatedAt.After(a.meta.UpdatedAt) {
			a.meta.UpdatedAt = r.UpdatedAt
		}
		if a.meta.Source == "" {
			a.meta.Source = r.Source
		}
		if a.meta.Category == "" {
			a.meta.Category = r.Category
		}
	}
	out := make([]model.ReputationListMeta, 0, len(by))
	for _, a := range by {
		out = append(out, a.meta)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
