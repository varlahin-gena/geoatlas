package huntadapter

import (
	"context"
	"fmt"
	"time"

	usecaseanomaly "geoatlas/internal/usecase/anomaly"
	"geoatlas/internal/usecase/hunts"
)

// HuntAnomalyReporter — synthetic hunt_threshold events.
type HuntAnomalyReporter struct {
	Anomaly *usecaseanomaly.Service
}

func (r HuntAnomalyReporter) ReportHuntBreach(ctx context.Context, hunt hunts.Hunt, run hunts.RunResult, now time.Time) error {
	if r.Anomaly == nil || !r.Anomaly.Available() {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	title := fmt.Sprintf("Hunt «%s»: %d рёбер", hunt.Name, run.EdgeCount)
	if run.PrevEdges > 0 && run.Ratio > 0 {
		title = fmt.Sprintf("Hunt «%s»: %d рёбер (×%.1f)", hunt.Name, run.EdgeCount, run.Ratio)
	}
	fp := usecaseanomaly.FingerprintForHunt(hunt.ID, now)
	ev := usecaseanomaly.Event{
		DetectedAt:  now,
		WindowStart: now.Add(-time.Hour),
		WindowEnd:   now,
		Code:        usecaseanomaly.CodeHuntThreshold,
		Severity:    usecaseanomaly.SeverityWarn,
		Score:       0.8,
		Title:       title,
		Detail: map[string]any{
			"hunt_id":    hunt.ID,
			"hunt_name":  hunt.Name,
			"edge_count": run.EdgeCount,
			"prev_edges": run.PrevEdges,
			"ratio":      run.Ratio,
			"query_cost": run.QueryCost,
		},
		EventCount: uint64(run.EdgeCount),
		Fingerprint: fp,
		ExpiresAt:  now.Add(30 * 24 * time.Hour),
		Map: usecaseanomaly.MapLink{
			Period:  hunt.Map.Period,
			Group:   hunt.Map.GroupBy,
			Filter:  hunt.Map.Filter,
			Query:   hunt.Map.Query,
			Country: hunt.Map.Country,
		},
	}
	return r.Anomaly.InsertSynthetic(ctx, []usecaseanomaly.Event{ev}, now)
}
