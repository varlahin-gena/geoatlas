package reputation

import (
	"net"
	"sort"
	"strings"
	"sync/atomic"

	"network_monitor/internal/model"
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

// Lookup возвращает все хиты (разные list/category) для IPv4.
func (i *Index) Lookup(ipStr string) []model.ReputationHit {
	parsed := net.ParseIP(strings.TrimSpace(ipStr))
	if parsed == nil || parsed.To4() == nil {
		return nil
	}
	ip := IPToUint32(ipStr)
	ranges := i.load()
	if len(ranges) == 0 {
		return nil
	}

	// Кандидаты: все с StartIP <= ip (бинарный поиск правой границы).
	pos := sort.Search(len(ranges), func(k int) bool {
		return ranges[k].StartIP > ip
	})
	seen := map[string]struct{}{}
	var hits []model.ReputationHit
	for k := 0; k < pos; k++ {
		r := ranges[k]
		if ip > r.EndIP {
			continue
		}
		key := r.ListName + "\x00" + r.Category
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		hits = append(hits, model.ReputationHit{List: r.ListName, Category: r.Category})
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
