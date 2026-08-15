package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"network_monitor/internal/adapter/httpapi/loginthrottle"
	"network_monitor/internal/adapter/searchtemplatesfile"
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

const (
	maxParseTestSize = 8 << 20 // 8 MiB — /api/parse-test
	maxJSONBodySize  = 1 << 20 // 1 MiB — мелкие JSON-эндпоинты

	healthTimeout = 5 * time.Second
	readTimeout   = 60 * time.Second // read-only эндпоинты
)

type Server struct {
	httpSrv *http.Server
	deps    *Deps
	routes  []RouteInfo
}

// ServerOption — опциональная настройка HTTP-сервера (метрики и т.п.).
type ServerOption func(*Deps)

func WithMetrics(m MetricsRecorder) ServerOption {
	return func(d *Deps) {
		if d != nil {
			d.WithMetrics(m)
		}
	}
}

func NewServer(
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
	authUC *usecaseauth.Service,
	users UserDirectory,
	sessions SessionParser,
	apiTokens APITokenStore,
	opts ...ServerOption,
) *Server {
	deps := NewDeps(cfg, ingestSvc, eventsUC, geoUC, reputationUC, parseErrorsUC, systemUC, systemPinger, parseTestUC, retentionUC, backupUC).
		WithAuth(authUC, users, sessions, apiTokens).
		WithSearchTemplates(searchtemplatesfile.New(cfg.SearchTemplatesFile))
	for _, opt := range opts {
		if opt != nil {
			opt(deps)
		}
	}
	health := &HealthHandler{deps}
	events := &EventsHandler{deps}
	ingestH := &IngestHandler{deps}
	geoH := &GeoHandler{deps}
	repH := &ReputationHandler{deps}
	system := &SystemHandler{deps}
	parse := &ParseHandler{deps}
	authH := &AuthHandler{deps}
	usersH := &UsersHandler{deps}
	tokensH := &APITokensHandler{deps}
	tplH := &SearchTemplatesHandler{deps}

	envTokens := cfg.APIAuthTokens()
	if cfg.APIAuthDisabled {
		envTokens = nil
	}
	ba := newBearerAuth(envTokens, apiTokens)
	uiAuthOff := cfg.AuthDisabled
	apiAuthOff := cfg.APIAuthDisabled

	loginMW := requireLoginMW(ba, sessions, users, uiAuthOff)
	adminMW := requireAdminMW(ba, sessions, users, uiAuthOff)
	// Pipeline ops: Bearer≥ops / administrator; open if AUTH_DISABLED или API_AUTH_DISABLED.
	opsMW := requireOpsMW(ba, sessions, users, apiAuthOff, uiAuthOff)
	csrf := csrfMW(ba, uiAuthOff)

	rr := newRouteRegistrar()

	// --- Auth (публичные / собственные) ---
	rr.Handle("POST", "/api/auth/login", chain(http.HandlerFunc(authH.Login), maxBytesMW(64<<10)))
	rr.Handle("POST", "/api/auth/logout",
		chain(http.HandlerFunc(authH.Logout), csrf),
	)
	rr.Handle("POST", "/api/auth/logout-all",
		chain(http.HandlerFunc(authH.LogoutAll), csrf),
	)
	rr.Handle("GET", "/api/auth/me", http.HandlerFunc(authH.Me))
	rr.Handle("POST", "/api/auth/change-password",
		chain(http.HandlerFunc(authH.ChangePassword), csrf, maxBytesMW(64<<10)),
	)
	rr.Handle("POST", "/api/auth/geo-wizard-dismiss",
		chain(http.HandlerFunc(authH.DismissGeoWizard), loginMW, csrf, maxBytesMW(64<<10)),
	)
	rr.Handle("GET", "/api/auth/check", http.HandlerFunc(authH.Check))
	rr.Handle("GET", "/api/auth/check-ops", http.HandlerFunc(authH.CheckOps))
	rr.Handle("GET", "/api/auth/check-admin", http.HandlerFunc(authH.CheckAdmin))

	// --- Управление УЗ (только администратор) ---
	rr.Handle("GET", "/api/users",
		withTimeout(chain(http.HandlerFunc(usersH.List), adminMW), healthTimeout),
	)
	rr.Handle("POST", "/api/users",
		chain(http.HandlerFunc(usersH.Create), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("POST", "/api/users/{username}/role",
		chain(http.HandlerFunc(usersH.SetRole), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("POST", "/api/users/{username}/full-name",
		chain(http.HandlerFunc(usersH.SetFullName), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("POST", "/api/users/{username}/reset-password",
		chain(http.HandlerFunc(usersH.ResetPassword), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("DELETE", "/api/users/{username}",
		chain(http.HandlerFunc(usersH.Delete), adminMW, csrf),
	)

	// --- Личные шаблоны поиска (карта) ---
	rr.Handle("GET", "/api/me/search-templates",
		withTimeout(chain(http.HandlerFunc(tplH.ListMine), loginMW), healthTimeout),
	)
	rr.Handle("POST", "/api/me/search-templates",
		chain(http.HandlerFunc(tplH.CreateMine), loginMW, csrf, maxBytesMW(64<<10)),
	)
	rr.Handle("PUT", "/api/me/search-templates/{id}",
		chain(http.HandlerFunc(tplH.UpdateMine), loginMW, csrf, maxBytesMW(64<<10)),
	)
	rr.Handle("DELETE", "/api/me/search-templates/{id}",
		chain(http.HandlerFunc(tplH.DeleteMine), loginMW, csrf),
	)
	rr.Handle("GET", "/api/search-templates",
		withTimeout(chain(http.HandlerFunc(tplH.ListAll), adminMW), healthTimeout),
	)

	// --- API-токены (administrator) ---
	rr.Handle("GET", "/api/tokens",
		withTimeout(chain(http.HandlerFunc(tokensH.List), adminMW), healthTimeout),
	)
	rr.Handle("POST", "/api/tokens",
		chain(http.HandlerFunc(tokensH.Create), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("DELETE", "/api/tokens/{id}",
		chain(http.HandlerFunc(tokensH.Revoke), adminMW, csrf),
	)

	// --- Probes открыты (docker/k8s). /live и /health — процесс; /ready — CH+ingest. ---
	rr.Handle("GET", "/live", withTimeout(http.HandlerFunc(health.Live), healthTimeout))
	rr.Handle("GET", "/api/live", withTimeout(http.HandlerFunc(health.Live), healthTimeout))
	rr.Handle("GET", "/health", withTimeout(http.HandlerFunc(health.Live), healthTimeout))
	rr.Handle("GET", "/api/health", withTimeout(http.HandlerFunc(health.Live), healthTimeout))
	rr.Handle("GET", "/ready", withTimeout(http.HandlerFunc(health.Ready), healthTimeout))
	rr.Handle("GET", "/api/ready", withTimeout(http.HandlerFunc(health.Ready), healthTimeout))
	rr.Handle("GET", "/api/ingest/stats",
		withTimeout(chain(http.HandlerFunc(ingestH.GetIngestStats), opsMW), healthTimeout),
	)
	// Prometheus scrape: Bearer≥ops / administrator (как ingest/stats).
	rr.Handle("GET", "/metrics", chain(metricsHandler(deps.prom), opsMW))

	// --- Карта / статус: любой залогиненный ---
	rr.Handle("GET", "/api/events",
		withTimeout(chain(http.HandlerFunc(events.GetEvents), loginMW), readTimeout),
	)
	rr.Handle("GET", "/api/events/series",
		withTimeout(chain(http.HandlerFunc(events.GetEventsSeries), loginMW), readTimeout),
	)
	rr.Handle("GET", "/api/system/status",
		withTimeout(chain(http.HandlerFunc(system.GetSystemStatus), loginMW), readTimeout),
	)
	rr.Handle("GET", "/api/system/version",
		withTimeout(chain(http.HandlerFunc(system.GetSystemVersion), loginMW), healthTimeout),
	)
	rr.Handle("GET", "/api/geo-missing",
		withTimeout(chain(http.HandlerFunc(geoH.GetGeoMissing), adminMW), readTimeout),
	)
	rr.Handle("GET", "/api/geo-ranges/export",
		withTimeout(chain(http.HandlerFunc(geoH.ExportGeoRangesCSV), opsMW), 10*time.Minute),
	)
	rr.Handle("POST", "/api/geo-ranges/clear",
		chain(http.HandlerFunc(geoH.ClearGeoRanges), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("GET", "/api/geo-ranges",
		withTimeout(chain(http.HandlerFunc(geoH.ListGeoRanges), adminMW), readTimeout),
	)
	rr.Handle("POST", "/api/geo-ranges",
		chain(http.HandlerFunc(geoH.AppendGeoRange), opsMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("PUT", "/api/geo-ranges",
		chain(http.HandlerFunc(geoH.UpdateGeoRange), opsMW, csrf, maxBytesMW(maxJSONBodySize)),
	)

	rr.Handle("GET", "/api/reputation/lists",
		withTimeout(chain(http.HandlerFunc(repH.ListLists), adminMW), readTimeout),
	)
	rr.Handle("DELETE", "/api/reputation/lists/{name}",
		chain(http.HandlerFunc(repH.DeleteList), opsMW, csrf),
	)
	rr.Handle("GET", "/api/reputation/feeds",
		withTimeout(chain(http.HandlerFunc(repH.ListFeeds), adminMW), readTimeout),
	)
	rr.Handle("POST", "/api/reputation/feeds",
		chain(http.HandlerFunc(repH.AddFeed), opsMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("DELETE", "/api/reputation/feeds/{name}",
		chain(http.HandlerFunc(repH.RemoveFeed), opsMW, csrf),
	)
	rr.Handle("GET", "/api/reputation/catalog",
		withTimeout(chain(http.HandlerFunc(repH.ListCatalog), adminMW), readTimeout),
	)
	rr.Handle("POST", "/api/reputation/refresh",
		chain(http.HandlerFunc(repH.Refresh), opsMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("GET", "/api/reputation/lookup",
		withTimeout(chain(http.HandlerFunc(repH.Lookup), loginMW), healthTimeout),
	)

	// --- Только администратор ---
	rr.Handle("GET", "/api/system/stats",
		withTimeout(chain(http.HandlerFunc(system.GetSystemStats), adminMW), readTimeout),
	)
	rr.Handle("GET", "/api/system/history",
		withTimeout(chain(http.HandlerFunc(system.GetSystemHistory), adminMW), readTimeout),
	)
	rr.Handle("GET", "/api/system/edges-agg",
		withTimeout(chain(http.HandlerFunc(system.GetEdgesAggStatus), adminMW), healthTimeout),
	)
	rr.Handle("POST", "/api/system/maintenance/backfill",
		chain(http.HandlerFunc(system.PostMaintenanceBackfill), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("GET", "/api/system/install-profile",
		withTimeout(chain(http.HandlerFunc(system.GetInstallProfile), adminMW), healthTimeout),
	)
	rr.Handle("GET", "/api/system/retention",
		withTimeout(chain(http.HandlerFunc(system.GetRetention), adminMW), healthTimeout),
	)
	rr.Handle("PUT", "/api/system/retention",
		chain(http.HandlerFunc(system.PutRetention), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("GET", "/api/system/backups",
		withTimeout(chain(http.HandlerFunc(system.GetBackups), adminMW), healthTimeout),
	)
	rr.Handle("POST", "/api/system/backups",
		chain(http.HandlerFunc(system.PostBackup), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("POST", "/api/system/backups/{name}/attach",
		chain(http.HandlerFunc(system.PostBackupAttach), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("POST", "/api/system/backups/{name}/detach",
		chain(http.HandlerFunc(system.PostBackupDetach), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("DELETE", "/api/system/backups/{name}",
		chain(http.HandlerFunc(system.DeleteBackup), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("GET", "/api/system/backup-schedule",
		withTimeout(chain(http.HandlerFunc(system.GetBackupSchedule), adminMW), healthTimeout),
	)
	rr.Handle("PUT", "/api/system/backup-schedule",
		chain(http.HandlerFunc(system.PutBackupSchedule), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)
	rr.Handle("GET", "/api/parse-errors",
		withTimeout(chain(http.HandlerFunc(parse.ListParseErrors), adminMW), readTimeout),
	)
	rr.Handle("GET", "/api/parse-samples",
		withTimeout(chain(http.HandlerFunc(parse.ParseSamples), adminMW), readTimeout),
	)
	rr.Handle("POST", "/api/parse-test",
		chain(http.HandlerFunc(parse.ParseTest), adminMW, csrf, maxBytesMW(maxParseTestSize)),
	)
	rr.Handle("POST", "/api/parse-errors/delete",
		chain(http.HandlerFunc(parse.DeleteParseErrors), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	)

	// --- Мутирующие: Bearer / administrator (не operator); open если *AUTH_DISABLED ---
	rr.Handle("POST", "/api/ingest",
		chain(http.HandlerFunc(ingestH.IngestLogs), opsMW, csrf, maxBytesMW(cfg.MaxLogUploadSize)),
	)
	rr.Handle("POST", "/upload-logs",
		chain(http.HandlerFunc(ingestH.UploadLogs), opsMW, csrf, maxBytesMW(cfg.MaxLogUploadSize)),
	)
	rr.Handle("POST", "/upload-geo",
		chain(http.HandlerFunc(geoH.UploadGeo), opsMW, csrf, maxBytesMW(cfg.MaxGeoUploadSize)),
	)
	rr.Handle("POST", "/upload-reputation",
		chain(http.HandlerFunc(repH.UploadReputation), opsMW, csrf, maxBytesMW(cfg.MaxReputationUploadSize)),
	)

	h := rr.Handler()
	h = loggingMW(h)
	if deps.prom != nil {
		h = metricsMW(deps.prom)(h)
	}
	h = recoverMW(h)
	h = requestIDMW(h) // outermost

	loginthrottle.ConfigureTrustedProxies(strings.Split(cfg.TrustedProxies, ","))

	return &Server{
		deps:   deps,
		routes: rr.Routes(),
		httpSrv: &http.Server{
			Addr:    cfg.ListenAddr,
			Handler: h,

			// Заголовки должны прийти быстро — защита от slowloris по заголовкам.
			ReadHeaderTimeout: 15 * time.Second,

			// Верхняя граница hung-connections. Должна перекрывать geo-импорт
			// (handler ctx 30m) и крупные upload; объём — MaxBytesReader + nginx.
			ReadTimeout:  35 * time.Minute,
			WriteTimeout: 35 * time.Minute,
			IdleTimeout:  120 * time.Second,
		},
	}
}

func (s *Server) ListenAndServe() error              { return s.httpSrv.ListenAndServe() }
func (s *Server) Shutdown(ctx context.Context) error { return s.httpSrv.Shutdown(ctx) }

// Handler returns the root HTTP handler (tests / smoke).
func (s *Server) Handler() http.Handler {
	if s == nil || s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Handler
}

// Routes returns registered method+path templates (for contract / auth-matrix tests).
func (s *Server) Routes() []RouteInfo {
	if s == nil {
		return nil
	}
	out := make([]RouteInfo, len(s.routes))
	copy(out, s.routes)
	return out
}
