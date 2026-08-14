package aggstate

import "sync"

var (
	hourlyEdgesReadyMu sync.RWMutex
	hourlyEdgesReady   bool
)

// PreferHourlyEdgesAgg — true после успешного backfill hourly IP-агрегата.
func PreferHourlyEdgesAgg() bool {
	hourlyEdgesReadyMu.RLock()
	defer hourlyEdgesReadyMu.RUnlock()
	return hourlyEdgesReady
}

// SetHourlyEdgesAggReady обновляет флаг (также для тестов).
func SetHourlyEdgesAggReady(v bool) {
	hourlyEdgesReadyMu.Lock()
	hourlyEdgesReady = v
	hourlyEdgesReadyMu.Unlock()
}
