package hunts

import (
	"context"
	"time"

	"geoatlas/internal/mapsearch"
)

// MapSnapshot — результат map query для hunt run.
type MapSnapshot struct {
	EdgeCount int
	RawPairs  int
	QueryCost mapsearch.QueryCostTier
	Skipped   string
}

// MapRunner — выполнение map query (adapter → events UC).
type MapRunner interface {
	RunMap(ctx context.Context, st MapState, timeout time.Duration) (MapSnapshot, error)
}

// AnomalyReporter — synthetic hunt_threshold alerts.
type AnomalyReporter interface {
	ReportHuntBreach(ctx context.Context, hunt Hunt, run RunResult, now time.Time) error
}

// Store — персистентность hunts.
type Store interface {
	List(username string) ([]Hunt, error)
	ListAll() ([]HuntWithAuthor, error)
	Create(username string, hunt Hunt) (Hunt, error)
	Update(username, id string, hunt Hunt) (Hunt, error)
	Delete(username, id string) error
	SaveRun(username, id string, hunt Hunt) error
}
