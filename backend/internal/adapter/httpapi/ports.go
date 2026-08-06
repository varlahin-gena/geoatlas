package httpapi

import (
	"context"
	"io"
	"time"

	"network_monitor/internal/adapter/searchtemplatesfile"
	"network_monitor/internal/auth"
	"network_monitor/internal/ingest"
	"network_monitor/internal/model"
	usecaseauth "network_monitor/internal/usecase/auth"
	usecaseevents "network_monitor/internal/usecase/events"
	usecasegeo "network_monitor/internal/usecase/geo"
	"network_monitor/internal/usecase/parseerrors"
	"network_monitor/internal/usecase/parsetest"
	usecasereputation "network_monitor/internal/usecase/reputation"
	usecaseretention "network_monitor/internal/usecase/retention"
	usecasesystem "network_monitor/internal/usecase/system"
)

// Ingester — live syslog ingest (реализация: *ingest.Service).
type Ingester interface {
	Stats() ingest.StatsSnapshot
	// FeedReader ставит строки в общую очередь workers (тот же backpressure, что TCP).
	FeedReader(ctx context.Context, r io.Reader, transport string) (model.IngestStats, error)
}

type EventsAPI interface {
	GetMap(context.Context, usecaseevents.GetMapInput) (usecaseevents.GetMapResult, error)
	GetSeries(context.Context, usecaseevents.GetSeriesInput) (usecaseevents.GetSeriesResult, error)
}

type GeoAPI interface {
	UploadCSV(context.Context, io.Reader, bool) (usecasegeo.UploadResult, error)
	ListMissing(context.Context, usecasegeo.ListMissingInput) (usecasegeo.ListMissingResult, error)
	FormatNetwork(uint32, uint32) string
	ListRanges(context.Context, usecasegeo.ListRangesInput) (usecasegeo.ListRangesResult, error)
	AppendRange(context.Context, string, string, string, string, float64, float64) (usecasegeo.MutateRangeResult, error)
	UpdateRange(context.Context, string, string, string, string, string, float64, float64) (usecasegeo.MutateRangeResult, error)
	ExportCSV(context.Context, io.Writer) error
}

type ReputationAPI interface {
	UploadCSV(context.Context, io.Reader, bool) (usecasereputation.UploadResult, error)
	ListLists(context.Context) ([]model.ReputationListMeta, error)
	DeleteList(context.Context, string) error
	Refresh(context.Context, bool) (usecasereputation.RefreshResult, error)
	Lookup(string) ([]model.ReputationHit, error)
	ListFeeds() ([]usecasereputation.Feed, error)
	ListCatalog() []usecasereputation.Feed
	AddFeed(context.Context, usecasereputation.Feed) error
	RemoveFeed(context.Context, string) error
}

type ParseErrorsAPI interface {
	List(context.Context, parseerrors.ListInput) (parseerrors.ListResult, error)
	Delete(context.Context, parseerrors.DeleteInput) error
}

type ParseTestAPI interface {
	Run(io.Reader) (parsetest.Result, error)
	Samples() map[string][]string
}

type SystemAPI interface {
	Health(context.Context, usecasesystem.ClickHousePinger) (usecasesystem.HealthResult, error)
	CollectStats(context.Context) (usecasesystem.SystemStatsResponse, error)
	Status(context.Context) (usecasesystem.SystemStatusResponse, error)
	History(context.Context, string, string) (usecasesystem.HistoryResponse, error)
	EdgesAgg(context.Context) usecasesystem.EdgesAggView
	ScheduleMaintenanceBackfill(context.Context) bool
	InstallProfile() (*usecasesystem.CapacityProfile, error)
	InstallMeta() usecasesystem.InstallMeta
}

type RetentionAPI interface {
	Get() (usecaseretention.Settings, error)
	Update(context.Context, usecaseretention.Settings) (usecaseretention.Settings, error)
}

type AuthAPI interface {
	Login(string, string) (usecaseauth.LoginResult, error)
	Me(string) (auth.UserPublic, error)
	SessionTTL() time.Duration
	ChangePassword(string, string, string) (auth.UserPublic, error)
	ListUsers() ([]auth.UserPublic, error)
	CreateUser(usecaseauth.CreateUserInput) (auth.UserPublic, error)
	SetRole(string, string) (auth.UserPublic, error)
	SetFullName(string, string) (auth.UserPublic, error)
	ResetPassword(usecaseauth.ResetPasswordInput) (auth.UserPublic, error)
	DeleteUser(string, string) error
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
	MustReset(string) bool
}
