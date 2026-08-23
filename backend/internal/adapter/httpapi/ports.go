package httpapi

import (
	"context"
	"io"
	"time"

	"geoatlas/internal/auth"
	"geoatlas/internal/model"
)

// Ingester — live syslog ingest (реализация: *ingestnet.Service).
type Ingester interface {
	Stats() model.IngestLiveStats
	// FeedReader ставит строки в общую очередь workers (тот же backpressure, что TCP).
	FeedReader(ctx context.Context, r io.Reader, transport string) (model.IngestStats, error)
}

type APITokenStore interface {
	List() []auth.APITokenPublic
	Create(string, string) (auth.APITokenPublic, string, error)
	Revoke(string) error
	Verify(string) (string, bool)
}

type SessionParser interface {
	Parse(string) (auth.Session, error)
	TTL() time.Duration
}

type UserDirectory interface {
	Get(string) (auth.UserPublic, bool)
	SessionVersion(string) (int64, bool)
	MustReset(string) bool
}
