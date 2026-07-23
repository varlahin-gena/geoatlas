package aggstate

import "sync"

var (
	geoEdgesReadyMu sync.RWMutex
	geoEdgesReady   bool
)

// PreferGeoEdgesAgg — true после успешного EnsureGeoEdgesAgg (city+country).
// Пока false, /api/events с group_by=city|country уходит в fallback, а не
// в частично заполненные traffic_edges_*_daily.
func PreferGeoEdgesAgg() bool {
	geoEdgesReadyMu.RLock()
	defer geoEdgesReadyMu.RUnlock()
	return geoEdgesReady
}

// SetGeoEdgesAggReady обновляет флаг готовности geo-edges (также для тестов).
func SetGeoEdgesAggReady(v bool) {
	geoEdgesReadyMu.Lock()
	geoEdgesReady = v
	geoEdgesReadyMu.Unlock()
}
