package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"geoatlas/internal/adapter/httpapi/loginthrottle"
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
			d.prom = m
		}
	}
}

func NewServer(p Params, opts ...ServerOption) *Server {
	deps := NewDeps(p)
	for _, opt := range opts {
		if opt != nil {
			opt(deps)
		}
	}
	health := &HealthHandler{deps.health}
	events := &EventsHandler{deps.events}
	ingestH := &IngestHandler{deps.ingest}
	geoH := &GeoHandler{deps.geo}
	repH := &ReputationHandler{deps.reputation}
	system := &SystemHandler{deps.system}
	parse := &ParseHandler{deps.parse}
	authDeps := deps.auth
	authH := &AuthHandler{authDeps}
	usersH := &UsersHandler{authDeps}
	tokensH := &APITokensHandler{authDeps}
	tplH := &SearchTemplatesHandler{deps.templates}
	anomH := &AnomalyHandler{deps.anomaly}
	huntsH := NewHuntsHandler(deps.hunts)

	cfg := p.Cfg
	envTokens := cfg.APIAuthTokens()
	if cfg.APIAuthDisabled {
		envTokens = nil
	}
	opsTokens := cfg.APIOpsTokens()
	if cfg.APIAuthDisabled {
		opsTokens = nil
	}
	ba := newBearerAuth(envTokens, opsTokens, p.APITokens)
	uiAuthOff := cfg.AuthDisabled
	apiAuthOff := cfg.APIAuthDisabled

	loginMW := requireLoginMW(ba, p.Sessions, p.Users, uiAuthOff)
	adminMW := requireAdminMW(ba, p.Sessions, p.Users, uiAuthOff)
	// Pipeline ops: Bearer≥ops / administrator; open if AUTH_DISABLED или API_AUTH_DISABLED.
	opsMW := requireOpsMW(ba, p.Sessions, p.Users, apiAuthOff, uiAuthOff)
	csrf := csrfMW(ba, uiAuthOff)

	rr := newRouteRegistrar()
	wiring := &routeWiring{
		rr:                  rr,
		health:              health,
		events:              events,
		ingest:              ingestH,
		geo:                 geoH,
		rep:                 repH,
		system:              system,
		parse:               parse,
		auth:                authH,
		users:               usersH,
		tokens:              tokensH,
		tpl:                 tplH,
		anom:                anomH,
		hunts:               huntsH,
		loginMW:             loginMW,
		adminMW:             adminMW,
		opsMW:               opsMW,
		csrf:                csrf,
		prom:                deps.prom,
		maxLogUpload:        cfg.MaxLogUploadSize,
		maxGeoUpload:        cfg.MaxGeoUploadSize,
		maxReputationUpload: cfg.MaxReputationUploadSize,
	}
	wiring.registerAll()

	h := rr.Handler()
	h = loggingMW(h)
	if deps.prom != nil {
		h = metricsMW(deps.prom)(h)
	}
	h = apiThreatMW(cfg)(h)
	proxyGateOn := cfg.RequireProxy && !uiAuthOff && !apiAuthOff
	h = proxyGateMW(ba, proxyGateOn)(h)
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
