package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"network_monitor/internal/adapter/httpapi/loginthrottle"
	"network_monitor/internal/config"
	usecaseanomaly "network_monitor/internal/usecase/anomaly"
	usecaseaudit "network_monitor/internal/usecase/auditlog"
	usecaseauth "network_monitor/internal/usecase/auth"
	usecasebackup "network_monitor/internal/usecase/backup"
	usecaseevents "network_monitor/internal/usecase/events"
	usecasegeo "network_monitor/internal/usecase/geo"
	"network_monitor/internal/usecase/parseerrors"
	"network_monitor/internal/usecase/parsetest"
	usecasereputation "network_monitor/internal/usecase/reputation"
	usecaseretention "network_monitor/internal/usecase/retention"
	"network_monitor/internal/usecase/searchtemplates"
	usecasesystem "network_monitor/internal/usecase/system"
)

// AuthDeps — зависимости auth/users/api-tokens handlers (без domain UC).
type AuthDeps struct {
	cfg          config.Config
	authUC       *usecaseauth.Service
	users        UserDirectory
	sessions     SessionParser
	apiTokens    APITokenStore
	loginLimiter *loginthrottle.Limiter
	logs         *usecaseaudit.Service
}

// SystemDeps — зависимости SystemHandler (system/retention/backup + shared loginLimiter).
type SystemDeps struct {
	cfg          config.Config
	systemUC     *usecasesystem.Service
	retentionUC  *usecaseretention.Service
	backupUC     *usecasebackup.Service
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
	cfg      config.Config
	eventsUC *usecaseevents.Service
	backupUC *usecasebackup.Service
}

// GeoDeps — зависимости GeoHandler.
type GeoDeps struct {
	cfg   config.Config
	geoUC *usecasegeo.Service
}

// IngestDeps — зависимости IngestHandler.
type IngestDeps struct {
	cfg    config.Config
	ingest Ingester
}

// ParseDeps — зависимости ParseHandler (parse-errors + parse-test).
type ParseDeps struct {
	cfg           config.Config
	parseErrorsUC *parseerrors.Service
	parseTestUC   *parsetest.Service
}

// ReputationDeps — зависимости ReputationHandler.
type ReputationDeps struct {
	reputationUC *usecasereputation.Service
}

// SearchTemplatesDeps — зависимости SearchTemplatesHandler.
type SearchTemplatesDeps struct {
	cfg             config.Config
	searchTemplates *searchtemplates.Service
	sessions        SessionParser
}

// AnomalyDeps — зависимости AnomalyHandler.
type AnomalyDeps struct {
	cfg       config.Config
	anomalyUC *usecaseanomaly.Service
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
	return &Deps{
		auth: &AuthDeps{
			cfg:          p.Cfg,
			authUC:       p.AuthUC,
			users:        p.Users,
			sessions:     p.Sessions,
			apiTokens:    p.APITokens,
			loginLimiter: lim,
			logs:         p.Logs,
		},
		system: &SystemDeps{
			cfg:          p.Cfg,
			systemUC:     p.SystemUC,
			retentionUC:  p.RetentionUC,
			backupUC:     p.BackupUC,
			loginLimiter: lim,
			logs:         p.Logs,
		},
		health:     &HealthDeps{systemUC: p.SystemUC, systemPinger: p.SystemPinger},
		events:     &EventsDeps{cfg: p.Cfg, eventsUC: p.EventsUC, backupUC: p.BackupUC},
		geo:        &GeoDeps{cfg: p.Cfg, geoUC: p.GeoUC},
		ingest:     &IngestDeps{cfg: p.Cfg, ingest: p.Ingest},
		parse:      &ParseDeps{cfg: p.Cfg, parseErrorsUC: p.ParseErrorsUC, parseTestUC: p.ParseTestUC},
		reputation: &ReputationDeps{reputationUC: p.ReputationUC},
		templates: &SearchTemplatesDeps{
			cfg:             p.Cfg,
			searchTemplates: p.SearchTemplatesUC,
			sessions:        p.Sessions,
		},
		anomaly: &AnomalyDeps{cfg: p.Cfg, anomalyUC: p.AnomalyUC},
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
