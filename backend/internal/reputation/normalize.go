package reputation

import (
	"sort"
	"strings"

	"geoatlas/internal/model"
)

// NormalizeRanges сортирует и сливает пересекающиеся/смежные диапазоны
// внутри одной пары (list_name, category). Разные списки не сливаются.
func NormalizeRanges(ranges []model.ReputationRange) []model.ReputationRange {
	if len(ranges) == 0 {
		return nil
	}
	type key struct{ list, cat string }
	groups := map[key][]model.ReputationRange{}
	for _, r := range ranges {
		r.ListName = strings.TrimSpace(r.ListName)
		r.Category = strings.TrimSpace(r.Category)
		if r.ListName == "" || r.EndIP < r.StartIP {
			continue
		}
		k := key{r.ListName, r.Category}
		groups[k] = append(groups[k], r)
	}
	out := make([]model.ReputationRange, 0, len(ranges))
	for _, g := range groups {
		sort.Slice(g, func(i, j int) bool {
			if g[i].StartIP != g[j].StartIP {
				return g[i].StartIP < g[j].StartIP
			}
			return g[i].EndIP < g[j].EndIP
		})
		cur := g[0]
		for i := 1; i < len(g); i++ {
			n := g[i]
			// пересечение или касание → merge
			if n.StartIP <= cur.EndIP+1 {
				if n.EndIP > cur.EndIP {
					cur.EndIP = n.EndIP
				}
				if n.UpdatedAt.After(cur.UpdatedAt) {
					cur.UpdatedAt = n.UpdatedAt
				}
				if cur.Source == "" {
					cur.Source = n.Source
				}
				continue
			}
			out = append(out, cur)
			cur = n
		}
		out = append(out, cur)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartIP != out[j].StartIP {
			return out[i].StartIP < out[j].StartIP
		}
		if out[i].EndIP != out[j].EndIP {
			return out[i].EndIP < out[j].EndIP
		}
		if out[i].ListName != out[j].ListName {
			return out[i].ListName < out[j].ListName
		}
		return out[i].Category < out[j].Category
	})
	return out
}
