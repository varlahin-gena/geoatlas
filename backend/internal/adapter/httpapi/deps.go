package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"network_monitor/internal/adapter/httpapi/loginthrottle"
	"network_monitor/internal/config"
	usecaseauth "network_monitor/internal/usecase/auth"
	usecasebackup "network_monitor/internal/usecase/backup"
	usecaseevents "network_monitor/internal/usecase/events"
	usecasegeo "network_monitor/internal/usecase/geo"
	"network_monitor/internal/usecase/parseerrors"
	"network_monitor/internal/usecase/parsetest"
	usecasereputation "network_monitor/internal/usecase/reputation"
	usecaseretention "network_monitor/internal/usecase/retention"
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
}

// NewAuthDeps собирает auth bag; lim=nil → default limiter.
func NewAuthDeps(
	cfg config.Config,
	authUC *usecaseauth.Service,
	users UserDirectory,
	sessions SessionParser,
	apiTokens APITokenStore,
	lim *loginthrottle.Limiter,
) *AuthDeps {
	if lim == nil {
		lim = loginthrottle.New(10, time.Minute, 5*time.Minute)
	}
	return &AuthDeps{
		cfg:          cfg,
		authUC:       authUC,
		users:        users,
		sessions:     sessions,
		apiTokens:    apiTokens,
		loginLimiter: lim,
	}
}

// Deps — общие зависимости HTTP-слоя (без clickhouse.Conn — только usecase/ports).
// Auth живёт в auth *AuthDeps (единственный источник limiter/sessions/UC).
type Deps struct {
	cfg             config.Config
	ingest          Ingester
	parseErrorsUC   *parseerrors.Service
	parseTestUC     *parsetest.Service
	eventsUC        *usecaseevents.Service
	geoUC           *usecasegeo.Service
	reputationUC    *usecasereputation.Service
	systemUC        *usecasesystem.Service
	systemPinger    usecasesystem.ClickHousePinger
	retentionUC     *usecaseretention.Service
	backupUC        *usecasebackup.Service
	auth            *AuthDeps
	searchTemplates SearchTemplatesStore
	prom            MetricsRecorder // optional Prometheus
}

// Auth возвращает owned AuthDeps (nil-safe).
func (d *Deps) Auth() *AuthDeps {
	if d == nil {
		return nil
	}
	return d.auth
}

// MetricsRecorder — HTTP + scrape handler (реализация: *metrics.Registry).
type MetricsRecorder interface {
	Handler() http.Handler
	ObserveHTTP(method, route string, status int, d time.Duration)
	IncInFlight()
	DecInFlight()
}

func NewDeps(
	cfg config.Config,
	ingestSvc Ingester,
	eventsUC *usecaseevents.Service,
	geoUC *usecasegeo.Service,
	reputationUC *usecasereputation.Service,
	parseErrorsUC *parseerrors.Service,
	systemUC *usecasesystem.Service,
	systemPinger usecasesystem.ClickHousePinger,
	parseTestUC *parsetest.Service,
	retentionUC *usecaseretention.Service,
	backupUC *usecasebackup.Service,
) *Deps {
	d := &Deps{
		cfg:           cfg,
		eventsUC:      eventsUC,
		geoUC:         geoUC,
		reputationUC:  reputationUC,
		parseErrorsUC: parseErrorsUC,
		parseTestUC:   parseTestUC,
		systemUC:      systemUC,
		systemPinger:  systemPinger,
		retentionUC:   retentionUC,
		backupUC:      backupUC,
		auth:          NewAuthDeps(cfg, nil, nil, nil, nil, nil),
	}
	if ingestSvc != nil {
		d.ingest = ingestSvc
	}
	return d
}

func (d *Deps) ensureAuth() *AuthDeps {
	if d == nil {
		return nil
	}
	if d.auth == nil {
		d.auth = NewAuthDeps(d.cfg, nil, nil, nil, nil, nil)
	}
	return d.auth
}

func (d *Deps) WithAuth(authUC *usecaseauth.Service, users UserDirectory, sessions SessionParser, apiTokens APITokenStore) *Deps {
	if d == nil {
		return nil
	}
	a := d.ensureAuth()
	a.cfg = d.cfg
	a.authUC = authUC
	a.users = users
	a.sessions = sessions
	a.apiTokens = apiTokens
	return d
}

func (d *Deps) WithSearchTemplates(store SearchTemplatesStore) *Deps {
	if d == nil {
		return nil
	}
	d.searchTemplates = store
	return d
}

func (d *Deps) WithLoginLimiter(lim *loginthrottle.Limiter) *Deps {
	if d == nil {
		return nil
	}
	a := d.ensureAuth()
	if lim != nil {
		a.loginLimiter = lim
	}
	return d
}

func (d *Deps) WithMetrics(m MetricsRecorder) *Deps {
	if d == nil {
		return nil
	}
	d.prom = m
	return d
}

type HealthHandler struct{ *Deps }
type EventsHandler struct{ *Deps }
type IngestHandler struct{ *Deps }
type GeoHandler struct{ *Deps }
type SystemHandler struct{ *Deps }
type ParseHandler struct{ *Deps }

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
