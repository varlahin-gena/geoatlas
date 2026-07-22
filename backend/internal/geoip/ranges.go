package geoip

import (
	"fmt"
	"sort"

	"network_monitor/internal/model"
)

// NormalizeRanges сортирует диапазоны и отбрасывает пересекающиеся / вложенные.
// Первый (меньший StartIP, затем меньший EndIP) сохраняется; последующие с
// StartIP <= prev.EndIP пропускаются. Так Lookup по prev-range остаётся корректным.
func NormalizeRanges(ranges []model.GeoRange) (clean []model.GeoRange, skipped int) {
	if len(ranges) == 0 {
		return nil, 0
	}
	out := append([]model.GeoRange(nil), ranges...)
	sort.Slice(out, func(a, b int) bool {
		if out[a].StartIP == out[b].StartIP {
			return out[a].EndIP < out[b].EndIP
		}
		return out[a].StartIP < out[b].StartIP
	})

	clean = make([]model.GeoRange, 0, len(out))
	for _, r := range out {
		if r.EndIP < r.StartIP {
			skipped++
			continue
		}
		if len(clean) > 0 && r.StartIP <= clean[len(clean)-1].EndIP {
			skipped++
			continue
		}
		clean = append(clean, r)
	}
	return clean, skipped
}

// CheckNonOverlapping возвращает ошибку, если после сортировки есть пересечения.
func CheckNonOverlapping(ranges []model.GeoRange) error {
	if len(ranges) == 0 {
		return nil
	}
	sorted := append([]model.GeoRange(nil), ranges...)
	sort.Slice(sorted, func(a, b int) bool {
		if sorted[a].StartIP == sorted[b].StartIP {
			return sorted[a].EndIP < sorted[b].EndIP
		}
		return sorted[a].StartIP < sorted[b].StartIP
	})
	for i := 1; i < len(sorted); i++ {
		prev, cur := sorted[i-1], sorted[i]
		if cur.EndIP < cur.StartIP {
			return fmt.Errorf("invalid geo range: end < start (%d > %d)", cur.StartIP, cur.EndIP)
		}
		if cur.StartIP <= prev.EndIP {
			return fmt.Errorf("overlapping geo ranges: [%d-%d] overlaps [%d-%d]",
				prev.StartIP, prev.EndIP, cur.StartIP, cur.EndIP)
		}
	}
	return nil
}
