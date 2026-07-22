package httpapi

import (
	"context"
	"io"

	"network_monitor/internal/ingest"
	"network_monitor/internal/model"
)

// Ingester — live syslog ingest (реализация: *ingest.Service).
type Ingester interface {
	Stats() ingest.StatsSnapshot
	// FeedReader ставит строки в общую очередь workers (тот же backpressure, что TCP).
	FeedReader(ctx context.Context, r io.Reader, transport string) (model.IngestStats, error)
}
