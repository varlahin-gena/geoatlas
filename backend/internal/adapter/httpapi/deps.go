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

// SystemDeps — зависимости SystemHandler (system/retention/backup + shared loginLimiter).
type SystemDeps struct {
	cfg          config.Config
	systemUC     *usecasesystem.Service
	retentionUC  *usecaseretention.Service
	backupUC     *usecasebackup.Service
	loginLimiter *loginthrottle.Limiter
}

// NewSystemDeps собирает system bag; lim обычно shared с AuthDeps.
func NewSystemDeps(
	cfg config.Config,
	systemUC *usecasesystem.Service,
	retentionUC *usecaseretention.Service,
	backupUC *usecasebackup.Service,
	lim *loginthrottle.Limiter,
) *SystemDeps {
	return &SystemDeps{
		cfg:          cfg,
		systemUC:     systemUC,
		retentionUC:  retentionUC,
		backupUC:     backupUC,
		loginLimiter: lim,
	}
}

// HealthDeps — зависимости HealthHandler (Ready: systemUC + CH pinger).
type HealthDeps struct {
	systemUC     *usecasesystem.Service
	systemPinger usecasesystem.ClickHousePinger
}

// NewHealthDeps собирает health bag.
func NewHealthDeps(systemUC *usecasesystem.Service, systemPinger usecasesystem.ClickHousePinger) *HealthDeps {
	return &HealthDeps{systemUC: systemUC, systemPinger: systemPinger}
}

// EventsDeps — зависимости EventsHandler (map/series + attached backup name).
type EventsDeps struct {
	cfg      config.Config
	eventsUC *usecaseevents.Service
	backupUC *usecasebackup.Service
}

// NewEventsDeps собирает events bag.
func NewEventsDeps(cfg config.Config, eventsUC *usecaseevents.Service, backupUC *usecasebackup.Service) *EventsDeps {
	return &EventsDeps{cfg: cfg, eventsUC: eventsUC, backupUC: backupUC}
}

// GeoDeps — зависимости GeoHandler.
type GeoDeps struct {
	cfg   config.Config
	geoUC *usecasegeo.Service
}

func NewGeoDeps(cfg config.Config, geoUC *usecasegeo.Service) *GeoDeps {
	return &GeoDeps{cfg: cfg, geoUC: geoUC}
}

// IngestDeps — зависимости IngestHandler.
type IngestDeps struct {
	cfg    config.Config
	ingest Ingester
}

func NewIngestDeps(cfg config.Config, ingest Ingester) *IngestDeps {
	return &IngestDeps{cfg: cfg, ingest: ingest}
}

// ParseDeps — зависимости ParseHandler (parse-errors + parse-test).
type ParseDeps struct {
	cfg           config.Config
	parseErrorsUC *parseerrors.Service
	parseTestUC   *parsetest.Service
}

func NewParseDeps(cfg config.Config, parseErrorsUC *parseerrors.Service, parseTestUC *parsetest.Service) *ParseDeps {
	return &ParseDeps{cfg: cfg, parseErrorsUC: parseErrorsUC, parseTestUC: parseTestUC}
}

// ReputationDeps — зависимости ReputationHandler.
type ReputationDeps struct {
	reputationUC *usecasereputation.Service
}

func NewReputationDeps(reputationUC *usecasereputation.Service) *ReputationDeps {
	return &ReputationDeps{reputationUC: reputationUC}
}

// SearchTemplatesDeps — зависимости SearchTemplatesHandler.
type SearchTemplatesDeps struct {
	cfg             config.Config
	searchTemplates SearchTemplatesStore
	sessions        SessionParser // shared с AuthDeps
}

func NewSearchTemplatesDeps(cfg config.Config, store SearchTemplatesStore, sessions SessionParser) *SearchTemplatesDeps {
	return &SearchTemplatesDeps{cfg: cfg, searchTemplates: store, sessions: sessions}
}

// Deps — композитор HTTP-слоя: владеет domain bags (без плоских UC-полей).
// Shared pointers: backupUC (system+events), systemUC (system+health), loginLimiter/sessions (auth+…).
type Deps struct {
	cfg         config.Config
	auth        *AuthDeps
	system      *SystemDeps
	health      *HealthDeps
	events      *EventsDeps
	geo         *GeoDeps
	ingest      *IngestDeps
	parse       *ParseDeps
	reputation  *ReputationDeps
	templates   *SearchTemplatesDeps
	prom        MetricsRecorder // optional Prometheus
}

func (d *Deps) Auth() *AuthDeps {
	if d == nil {
		return nil
	}
	return d.auth
}

func (d *Deps) System() *SystemDeps {
	if d == nil {
		return nil
	}
	return d.system
}

func (d *Deps) Health() *HealthDeps {
	if d == nil {
		return nil
	}
	return d.health
}

func (d *Deps) Events() *EventsDeps {
	if d == nil {
		return nil
	}
	return d.events
}

func (d *Deps) Geo() *GeoDeps {
	if d == nil {
		return nil
	}
	return d.geo
}

func (d *Deps) Ingest() *IngestDeps {
	if d == nil {
		return nil
	}
	return d.ingest
}

func (d *Deps) Parse() *ParseDeps {
	if d == nil {
		return nil
	}
	return d.parse
}

func (d *Deps) Reputation() *ReputationDeps {
	if d == nil {
		return nil
	}
	return d.reputation
}

func (d *Deps) SearchTemplates() *SearchTemplatesDeps {
	if d == nil {
		return nil
	}
	return d.templates
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
	auth := NewAuthDeps(cfg, nil, nil, nil, nil, nil)
	return &Deps{
		cfg:        cfg,
		auth:       auth,
		system:     NewSystemDeps(cfg, systemUC, retentionUC, backupUC, auth.loginLimiter),
		health:     NewHealthDeps(systemUC, systemPinger),
		events:     NewEventsDeps(cfg, eventsUC, backupUC),
		geo:        NewGeoDeps(cfg, geoUC),
		ingest:     NewIngestDeps(cfg, ingestSvc),
		parse:      NewParseDeps(cfg, parseErrorsUC, parseTestUC),
		reputation: NewReputationDeps(reputationUC),
		templates:  NewSearchTemplatesDeps(cfg, nil, nil),
	}
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

func (d *Deps) ensureSystem() *SystemDeps {
	if d == nil {
		return nil
	}
	if d.system == nil {
		var lim *loginthrottle.Limiter
		if a := d.ensureAuth(); a != nil {
			lim = a.loginLimiter
		}
		d.system = NewSystemDeps(d.cfg, nil, nil, nil, lim)
	}
	return d.system
}

func (d *Deps) ensureTemplates() *SearchTemplatesDeps {
	if d == nil {
		return nil
	}
	if d.templates == nil {
		d.templates = NewSearchTemplatesDeps(d.cfg, nil, nil)
	}
	return d.templates
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
	if t := d.ensureTemplates(); t != nil {
		t.sessions = sessions
		t.cfg = d.cfg
	}
	if s := d.ensureSystem(); s != nil {
		s.loginLimiter = a.loginLimiter
		s.cfg = d.cfg
	}
	return d
}

func (d *Deps) WithSearchTemplates(store SearchTemplatesStore) *Deps {
	if d == nil {
		return nil
	}
	t := d.ensureTemplates()
	t.searchTemplates = store
	t.cfg = d.cfg
	if d.auth != nil {
		t.sessions = d.auth.sessions
	}
	return d
}

func (d *Deps) WithLoginLimiter(lim *loginthrottle.Limiter) *Deps {
	if d == nil {
		return nil
	}
	if lim == nil {
		return d
	}
	a := d.ensureAuth()
	a.loginLimiter = lim
	if s := d.ensureSystem(); s != nil {
		s.loginLimiter = lim
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
