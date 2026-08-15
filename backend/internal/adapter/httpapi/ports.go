package httpapi

import (
	"context"
	"io"
	"time"

	"network_monitor/internal/adapter/searchtemplatesfile"
	"network_monitor/internal/auth"
	"network_monitor/internal/adapter/ingestnet"
	"network_monitor/internal/model"
)

// Ingester — live syslog ingest (реализация: *ingestnet.Service).
type Ingester interface {
	Stats() ingestnet.StatsSnapshot
	// FeedReader ставит строки в общую очередь workers (тот же backpressure, что TCP).
	FeedReader(ctx context.Context, r io.Reader, transport string) (model.IngestStats, error)
}

type APITokenStore interface {
	List() []auth.APITokenPublic
	Create(string, string) (auth.APITokenPublic, string, error)
	Revoke(string) error
	Verify(string) (string, bool)
}

type SearchTemplatesStore interface {
	List(string) ([]searchtemplatesfile.Template, error)
	Create(string, string, string) (searchtemplatesfile.Template, error)
	Update(string, string, string, string) (searchtemplatesfile.Template, error)
	Delete(string, string) error
	ListAll() ([]searchtemplatesfile.TemplateWithAuthor, error)
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
