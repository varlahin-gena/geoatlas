package aggstate

import (
	"sync"
	"time"
)

// EdgesAggStatus — состояние Ensure*/backfill для API и логов.
// PreferDailyEdgesAgg() == true только при State == "ready".
type EdgesAggStatus struct {
	State     string    `json:"state"`           // idle | pending | running | ready | error
	Phase     string    `json:"phase,omitempty"` // schema | backfill (только при running)
	Message   string    `json:"message"`
	RawRows   uint64    `json:"raw_rows"`
	AggRows   uint64    `json:"agg_rows"`
	DaysTotal int       `json:"days_total"`
	DaysDone  int       `json:"days_done"`
	StartedAt time.Time `json:"started_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

const (
	PhaseSchema   = "schema"
	PhaseBackfill = "backfill"
)

var (
	edgesAggMu     sync.RWMutex
	edgesAggStatus = EdgesAggStatus{State: "idle", Message: "not started"}
)

// SetEdgesAggStatus обновляет статус IP-edges backfill (также для тестов).
func SetEdgesAggStatus(st EdgesAggStatus) {
	st.UpdatedAt = time.Now().UTC()
	edgesAggMu.Lock()
	edgesAggStatus = st
	edgesAggMu.Unlock()
}

func GetEdgesAggStatus() EdgesAggStatus {
	edgesAggMu.RLock()
	defer edgesAggMu.RUnlock()
	return edgesAggStatus
}

// PreferDailyEdgesAgg — true только после успешного полного backfill.
// Пока State != ready (idle/running/error), API читает traffic_logs, чтобы
// не отдавать карту с пропущенными днями из частичного агрегата / после DROP MV.
func PreferDailyEdgesAgg() bool {
	return GetEdgesAggStatus().State == "ready"
}
