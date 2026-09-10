package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"geoatlas/internal/adapter/httpapi/loginthrottle"
	"geoatlas/internal/config"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
	usecaseaudit "geoatlas/internal/usecase/auditlog"
	usecaseauth "geoatlas/internal/usecase/auth"
	usecasebackup "geoatlas/internal/usecase/backup"
	usecaseevents "geoatlas/internal/usecase/events"
	usecasegeo "geoatlas/internal/usecase/geo"
	usecasehunts "geoatlas/internal/usecase/hunts"
	"geoatlas/internal/usecase/parseerrors"
	"geoatlas/internal/usecase/parsetest"
	usecasereputation "geoatlas/internal/usecase/reputation"
	usecaseretention "geoatlas/internal/usecase/retention"
	"geoatlas/internal/usecase/searchtemplates"
	usecasesystem "geoatlas/internal/usecase/system"
	usecasetls "geoatlas/internal/usecase/tls"
)

// AuthDeps — зависимости auth/users/api-tokens handlers (без domain UC).
type AuthDeps struct {
	authDisabled      bool
	apiAuthDisabled   bool
	reputationEnabled bool
	apiAuthTokens     []string
	apiOpsTokens      []string
	authUC            *usecaseauth.Service
	users             UserDirectory
	sessions          SessionParser
	apiTokens         APITokenStore
	reauth            ReauthChecker
	loginLimiter      *loginthrottle.Limiter
	logs              *usecaseaudit.Service
}

// SystemDeps — зависимости SystemHandler (system/retention/backup + shared loginLimiter).
type SystemDeps struct {
	queryTimeout time.Duration
	systemUC     *usecasesystem.Service
	retentionUC  *usecaseretention.Service
	backupUC     *usecasebackup.Service
	tlsUC        *usecasetls.Service
	reauth       ReauthChecker
	loginLimiter *loginthrottle.Limiter
	logs         *usecaseaudit.Service
}

// HealthDeps — зависимости HealthHandler (Ready: systemUC + CH pinger).
type HealthDeps struct {
	systemUC     *usecasesystem.Service
	systemPinger usecasesystem.ClickHousePinger
}

// EventsDeps — зависимости EventsHandler (map/series + attached backup name).
type EventsDeps struct {
	queryTimeout time.Duration
	eventsUC     *usecaseevents.Service
	backupUC     *usecasebackup.Service
}

// GeoDeps — зависимости GeoHandler.
type GeoDeps struct {
	maxGeoUploadSize   int64
	maxGeoUploadRanges int
	queryTimeout       time.Duration
	geoUC              *usecasegeo.Service
}

// IngestDeps — зависимости IngestHandler.
type IngestDeps struct {
	ingestFlushSec int
	ingest         Ingester
}

// ParseDeps — зависимости ParseHandler (parse-errors + parse-test).
type ParseDeps struct {
	queryTimeout  time.Duration
	parseErrorsUC *parseerrors.Service
	parseTestUC   *parsetest.Service
}

// ReputationDeps — зависимости ReputationHandler.
type ReputationDeps struct {
	reputationUC *usecasereputation.Service
}

// SearchTemplatesDeps — зависимости SearchTemplatesHandler.
type SearchTemplatesDeps struct {
	authDisabled    bool
	searchTemplates *searchtemplates.Service
	sessions        SessionParser
}

// AnomalyDeps — зависимости AnomalyHandler.
type AnomalyDeps struct {
	queryTimeout    time.Duration
	authDisabled    bool
	anomalyUC       *usecaseanomaly.Service
	anomalySettings *usecaseanomaly.SettingsService
	logs            *usecaseaudit.Service
}

// Params — вход NewDeps / NewServer.
type Params struct {
	Cfg               config.Config
	Ingest            Ingester
	EventsUC          *usecaseevents.Service
	GeoUC             *usecasegeo.Service
	ReputationUC      *usecasereputation.Service
	ParseErrorsUC     *parseerrors.Service
	SystemUC          *usecasesystem.Service
	SystemPinger      usecasesystem.ClickHousePinger
	ParseTestUC       *parsetest.Service
	RetentionUC       *usecaseretention.Service
	BackupUC          *usecasebackup.Service
	AuthUC            *usecaseauth.Service
	Users             UserDirectory
	Sessions          SessionParser
	APITokens         APITokenStore
	SearchTemplatesUC *searchtemplates.Service
	AnomalyUC         *usecaseanomaly.Service
	AnomalySettingsUC *usecaseanomaly.SettingsService
	HuntsUC           *usecasehunts.Service
	Logs              *usecaseaudit.Service
}

// Deps — композитор HTTP-слоя: domain bags без плоских UC-полей.
// Shared pointers: BackupUC (system+events), SystemUC (system+health),
// loginLimiter/Sessions (auth+templates+system).
type Deps struct {
	auth       *AuthDeps
	system     *SystemDeps
	health     *HealthDeps
	events     *EventsDeps
	geo        *GeoDeps
	ingest     *IngestDeps
	parse      *ParseDeps
	reputation *ReputationDeps
	templates  *SearchTemplatesDeps
	anomaly    *AnomalyDeps
	hunts      *usecasehunts.Service
	prom       MetricsRecorder
}

// MetricsRecorder — HTTP + scrape handler (реализация: *metrics.Registry).
type MetricsRecorder interface {
	Handler() http.Handler
	ObserveHTTP(method, route string, status int, d time.Duration)
	IncInFlight()
	DecInFlight()
}

func NewDeps(p Params) *Deps {
	lim := loginthrottle.New(10, time.Minute, 5*time.Minute)
	reauth := NewReauthChecker(p.Cfg, p.AuthUC, p.Sessions, p.APITokens)
	envTokens := p.Cfg.APIAuthTokens()
	opsTokens := p.Cfg.APIOpsTokens()
	if p.Cfg.Auth.APIAuthDisabled {
		envTokens = nil
		opsTokens = nil
	}
	return &Deps{
		auth: &AuthDeps{
			authDisabled:      p.Cfg.Auth.Disabled,
			apiAuthDisabled:   p.Cfg.Auth.APIAuthDisabled,
			reputationEnabled: p.Cfg.Reputation.FetchEnabled,
			apiAuthTokens:     envTokens,
			apiOpsTokens:      opsTokens,
			authUC:            p.AuthUC,
			users:             p.Users,
			sessions:          p.Sessions,
			apiTokens:         p.APITokens,
			reauth:            reauth,
			loginLimiter:      lim,
			logs:              p.Logs,
		},
		system: &SystemDeps{
			queryTimeout: p.Cfg.QueryTimeout,
			systemUC:     p.SystemUC,
			retentionUC:  p.RetentionUC,
			backupUC:     p.BackupUC,
			reauth:       reauth,
			tlsUC:        usecasetls.New(usecasetls.Config(p.Cfg.TLS)),
			loginLimiter: lim,
			logs:         p.Logs,
		},
		health: &HealthDeps{systemUC: p.SystemUC, systemPinger: p.SystemPinger},
		events: &EventsDeps{queryTimeout: p.Cfg.QueryTimeout, eventsUC: p.EventsUC, backupUC: p.BackupUC},
		geo: &GeoDeps{
			maxGeoUploadSize:   p.Cfg.Geo.MaxUploadSize,
			maxGeoUploadRanges: p.Cfg.Geo.MaxUploadRanges,
			queryTimeout:       p.Cfg.QueryTimeout,
			geoUC:              p.GeoUC,
		},
		ingest:     &IngestDeps{ingestFlushSec: p.Cfg.Ingest.FlushSec, ingest: p.Ingest},
		parse:      &ParseDeps{queryTimeout: p.Cfg.QueryTimeout, parseErrorsUC: p.ParseErrorsUC, parseTestUC: p.ParseTestUC},
		reputation: &ReputationDeps{reputationUC: p.ReputationUC},
		templates: &SearchTemplatesDeps{
			authDisabled:    p.Cfg.Auth.Disabled,
			searchTemplates: p.SearchTemplatesUC,
			sessions:        p.Sessions,
		},
		anomaly: &AnomalyDeps{
			queryTimeout:    p.Cfg.QueryTimeout,
			authDisabled:    p.Cfg.Auth.Disabled,
			anomalyUC:       p.AnomalyUC,
			anomalySettings: p.AnomalySettingsUC,
			logs:            p.Logs,
		},
		hunts: p.HuntsUC,
	}
}

type HealthHandler struct{ *HealthDeps }
type EventsHandler struct{ *EventsDeps }
type IngestHandler struct{ *IngestDeps }
type GeoHandler struct{ *GeoDeps }
type SystemHandler struct{ *SystemDeps }
type ParseHandler struct{ *ParseDeps }

func metricsHandler(m MetricsRecorder) http.Handler {
	if m != nil {
		return m.Handler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "metrics not configured"})
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("writeJSON: marshal failed", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"json marshal failed"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeInternalError(w http.ResponseWriter, logMsg string, err error) {
	slog.Error(logMsg, "err", err)
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal server error"})
}
