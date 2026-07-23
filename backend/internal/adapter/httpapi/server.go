package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"network_monitor/internal/auth"
	"network_monitor/internal/config"
	usecaseauth "network_monitor/internal/usecase/auth"
	usecaseevents "network_monitor/internal/usecase/events"
	usecasegeo "network_monitor/internal/usecase/geo"
	"network_monitor/internal/usecase/parseerrors"
	"network_monitor/internal/usecase/parsetest"
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
}

func NewServer(
	cfg config.Config,
	ingestSvc Ingester,
	eventsUC *usecaseevents.Service,
	geoUC *usecasegeo.Service,
	parseErrorsUC *parseerrors.Service,
	systemUC *usecasesystem.Service,
	systemPinger usecasesystem.ClickHousePinger,
	parseTestUC *parsetest.Service,
	retentionUC *usecaseretention.Service,
	authUC *usecaseauth.Service,
	users *auth.UserStore,
	sessions *auth.SessionManager,
	apiTokens *auth.TokenStore,
) *Server {
	deps := NewDeps(cfg, ingestSvc, eventsUC, geoUC, parseErrorsUC, systemUC, systemPinger, parseTestUC, retentionUC).
		WithAuth(authUC, users, sessions, apiTokens)
	health := &HealthHandler{deps}
	events := &EventsHandler{deps}
	ingestH := &IngestHandler{deps}
	geoH := &GeoHandler{deps}
	system := &SystemHandler{deps}
	parse := &ParseHandler{deps}
	authH := &AuthHandler{deps}
	usersH := &UsersHandler{deps}
	tokensH := &APITokensHandler{deps}

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

	r := mux.NewRouter()

	// Глобальные middleware: паника -> 500, access-логи.
	r.Use(requestIDMW)
	r.Use(recoverMW)
	r.Use(loggingMW)

	// --- Auth (публичные / собственные) ---
	r.Handle("/api/auth/login", chain(http.HandlerFunc(authH.Login), maxBytesMW(64<<10))).Methods("POST")
	r.Handle("/api/auth/logout",
		chain(http.HandlerFunc(authH.Logout), csrf),
	).Methods("POST")
	r.HandleFunc("/api/auth/me", authH.Me).Methods("GET")
	r.Handle("/api/auth/change-password",
		chain(http.HandlerFunc(authH.ChangePassword), csrf, maxBytesMW(64<<10)),
	).Methods("POST")
	r.HandleFunc("/api/auth/check", authH.Check).Methods("GET")
	r.HandleFunc("/api/auth/check-admin", authH.CheckAdmin).Methods("GET")

	// --- Управление УЗ (только администратор) ---
	r.Handle("/api/users",
		withTimeout(chain(http.HandlerFunc(usersH.List), adminMW), healthTimeout),
	).Methods("GET")
	r.Handle("/api/users",
		chain(http.HandlerFunc(usersH.Create), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	).Methods("POST")
	r.Handle("/api/users/{username}/role",
		chain(http.HandlerFunc(usersH.SetRole), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	).Methods("POST")
	r.Handle("/api/users/{username}/full-name",
		chain(http.HandlerFunc(usersH.SetFullName), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	).Methods("POST")
	r.Handle("/api/users/{username}/reset-password",
		chain(http.HandlerFunc(usersH.ResetPassword), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	).Methods("POST")
	r.Handle("/api/users/{username}",
		chain(http.HandlerFunc(usersH.Delete), adminMW, csrf),
	).Methods("DELETE")

	// --- API-токены (administrator) ---
	r.Handle("/api/tokens",
		withTimeout(chain(http.HandlerFunc(tokensH.List), adminMW), healthTimeout),
	).Methods("GET")
	r.Handle("/api/tokens",
		chain(http.HandlerFunc(tokensH.Create), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	).Methods("POST")
	r.Handle("/api/tokens/{id}",
		chain(http.HandlerFunc(tokensH.Revoke), adminMW, csrf),
	).Methods("DELETE")

	// --- Health открыт (docker/k8s probes). Pipeline stats — opsMW. ---
	r.Handle("/health", withTimeout(http.HandlerFunc(health.Health), healthTimeout)).Methods("GET")
	r.Handle("/api/health", withTimeout(http.HandlerFunc(health.Health), healthTimeout)).Methods("GET")
	r.Handle("/api/ingest/stats",
		withTimeout(chain(http.HandlerFunc(ingestH.GetIngestStats), opsMW), healthTimeout),
	).Methods("GET")

	// --- Карта / статус: любой залогиненный ---
	r.Handle("/api/events",
		withTimeout(chain(http.HandlerFunc(events.GetEvents), loginMW), readTimeout),
	).Methods("GET")
	r.Handle("/api/system/status",
		withTimeout(chain(http.HandlerFunc(system.GetSystemStatus), loginMW), readTimeout),
	).Methods("GET")
	r.Handle("/api/geo-missing",
		withTimeout(chain(http.HandlerFunc(geoH.GetGeoMissing), adminMW), readTimeout),
	).Methods("GET")
	r.Handle("/api/geo-ranges/export",
		withTimeout(chain(http.HandlerFunc(geoH.ExportGeoRangesCSV), opsMW), 10*time.Minute),
	).Methods("GET")
	r.Handle("/api/geo-ranges",
		withTimeout(chain(http.HandlerFunc(geoH.ListGeoRanges), adminMW), readTimeout),
	).Methods("GET")
	r.Handle("/api/geo-ranges",
		chain(http.HandlerFunc(geoH.AppendGeoRange), opsMW, csrf, maxBytesMW(maxJSONBodySize)),
	).Methods("POST")
	r.Handle("/api/geo-ranges",
		chain(http.HandlerFunc(geoH.UpdateGeoRange), opsMW, csrf, maxBytesMW(maxJSONBodySize)),
	).Methods("PUT")

	// --- Только администратор ---
	r.Handle("/api/system/stats",
		withTimeout(chain(http.HandlerFunc(system.GetSystemStats), adminMW), readTimeout),
	).Methods("GET")
	r.Handle("/api/system/history",
		withTimeout(chain(http.HandlerFunc(system.GetSystemHistory), adminMW), readTimeout),
	).Methods("GET")
	r.Handle("/api/system/edges-agg",
		withTimeout(chain(http.HandlerFunc(system.GetEdgesAggStatus), adminMW), healthTimeout),
	).Methods("GET")
	r.Handle("/api/system/maintenance/backfill",
		chain(http.HandlerFunc(system.PostMaintenanceBackfill), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	).Methods("POST")
	r.Handle("/api/system/install-profile",
		withTimeout(chain(http.HandlerFunc(system.GetInstallProfile), adminMW), healthTimeout),
	).Methods("GET")
	r.Handle("/api/system/retention",
		withTimeout(chain(http.HandlerFunc(system.GetRetention), adminMW), healthTimeout),
	).Methods("GET")
	r.Handle("/api/system/retention",
		chain(http.HandlerFunc(system.PutRetention), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	).Methods("PUT")
	r.Handle("/api/parse-errors",
		withTimeout(chain(http.HandlerFunc(parse.ListParseErrors), adminMW), readTimeout),
	).Methods("GET")
	r.Handle("/api/parse-samples",
		withTimeout(chain(http.HandlerFunc(parse.ParseSamples), adminMW), readTimeout),
	).Methods("GET")
	r.Handle("/api/parse-test",
		chain(http.HandlerFunc(parse.ParseTest), adminMW, csrf, maxBytesMW(maxParseTestSize)),
	).Methods("POST")
	r.Handle("/api/parse-errors/delete",
		chain(http.HandlerFunc(parse.DeleteParseErrors), adminMW, csrf, maxBytesMW(maxJSONBodySize)),
	).Methods("POST")

	// --- Мутирующие: Bearer / administrator (не operator); open если *AUTH_DISABLED ---
	r.Handle("/api/ingest",
		chain(http.HandlerFunc(ingestH.IngestLogs), opsMW, csrf, maxBytesMW(cfg.MaxLogUploadSize)),
	).Methods("POST")
	r.Handle("/upload-logs",
		chain(http.HandlerFunc(ingestH.UploadLogs), opsMW, csrf, maxBytesMW(cfg.MaxLogUploadSize)),
	).Methods("POST")
	r.Handle("/upload-geo",
		chain(http.HandlerFunc(geoH.UploadGeo), opsMW, csrf, maxBytesMW(cfg.MaxGeoUploadSize)),
	).Methods("POST")

	return &Server{
		deps: deps,
		httpSrv: &http.Server{
			Addr:    cfg.ListenAddr,
			Handler: r,

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
