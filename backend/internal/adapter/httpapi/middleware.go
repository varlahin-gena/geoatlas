package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"geoatlas/internal/adapter/httpapi/authmw"
	"geoatlas/internal/adapter/httpapi/loginthrottle"
	"geoatlas/internal/auth"
)

type ctxKey int

const (
	requestIDCtxKey    ctxKey = 2
	routePatternCtxKey ctxKey = 3
)

const requestIDHeader = "X-Request-ID"

// SessionFromContext — live session from authmw Require* middleware.
func SessionFromContext(ctx context.Context) (auth.Session, bool) {
	return authmw.SessionFromContext(ctx)
}

type middleware func(http.Handler) http.Handler

// chain оборачивает handler набором middleware: первый в списке — внешний.
func chain(h http.Handler, mws ...middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// recoverMW ловит панику в хендлере, логирует стек и отвечает 500.
func recoverMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture before defer: contextcheck flags RequestIDFromContext(r.Context()) inside nested defer.
		reqID := RequestIDFromContext(r.Context())
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic",
					"request_id", reqID,
					"method", r.Method,
					"path", r.URL.Path,
					"err", rec,
					"stack", string(debug.Stack()),
				)
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// requestIDMW принимает/генерирует X-Request-ID, кладёт в context и ответ.
func requestIDMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if id == "" || len(id) > 128 || strings.ContainsAny(id, "\r\n") {
			id = newRequestID()
		}
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(withRequestID(r.Context(), id)))
	})
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format("150405.000000000")))
	}
	return hex.EncodeToString(b[:])
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(requestIDCtxKey).(string)
	return id
}

func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDCtxKey, id)
}

// statusRecorder перехватывает код ответа и объём для request-лога.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status = code
		s.wrote = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wrote {
		s.status = http.StatusOK
		s.wrote = true
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

// withRoutePattern кладёт path template в context для низкокардинальных лейблов.
func withRoutePattern(pattern string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routePatternCtxKey, pattern)))
	})
}

// routeLabel возвращает path template (низкая cardinality), иначе "unmatched".
func routeLabel(r *http.Request) string {
	if r == nil {
		return "unmatched"
	}
	if p, ok := r.Context().Value(routePatternCtxKey).(string); ok && p != "" {
		return p
	}
	// ServeMux мутирует r.Pattern на исходном *Request (например "GET /api/events").
	if pat := strings.TrimSpace(r.Pattern); pat != "" {
		if _, path, ok := strings.Cut(pat, " "); ok && path != "" {
			return path
		}
		return pat
	}
	return "unmatched"
}

// loggingMW пишет access-лог с latency.
func loggingMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start)
		slog.Info("http",
			"request_id", RequestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"route", routeLabel(r),
			"status", rec.status,
			"bytes", rec.bytes,
			"duration", elapsed.Round(time.Millisecond).String(),
		)
	})
}

// metricsMW пишет Prometheus HTTP latency (route template — низкая cardinality).
func metricsMW(m MetricsRecorder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if m == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Не считаем scrape самого /metrics — иначе self-noise.
			if r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			m.IncInFlight()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			m.DecInFlight()
			m.ObserveHTTP(r.Method, routeLabel(r), rec.status, time.Since(start))
		})
	}
}

// maxBytesMW ограничивает размер тела запроса.
func maxBytesMW(n int64) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

// Thin wrappers keep server/tests on httpapi names while auth lives in authmw.

func newBearerAuth(envAdmin, envOps []string, store APITokenStore) authmw.BearerAuth {
	return authmw.NewBearerAuth(envAdmin, envOps, store)
}

func sessionLoader(sessions SessionParser) authmw.SessionLoader {
	return func(r *http.Request) (auth.Session, error) {
		return SessionFromRequest(r, sessions)
	}
}

func requireLoginMW(ba authmw.BearerAuth, sessions SessionParser, users UserDirectory, authDisabled bool) middleware {
	return middleware(authmw.RequireLogin(ba, sessionLoader(sessions), users, authDisabled))
}

func requireAdminMW(ba authmw.BearerAuth, sessions SessionParser, users UserDirectory, authDisabled bool) middleware {
	return middleware(authmw.RequireAdmin(ba, sessionLoader(sessions), users, authDisabled))
}

func requireOpsMW(ba authmw.BearerAuth, sessions SessionParser, users UserDirectory, apiAuthDisabled, authDisabled bool) middleware {
	return middleware(authmw.RequireOps(ba, sessionLoader(sessions), users, apiAuthDisabled, authDisabled))
}

// proxyGateMW — браузерный доступ только через nginx (GA_TRUSTED_PROXIES / loopback).
// Прямой доступ с хоста к :8080 отклоняется; machine clients с валидным Bearer — ок.
// Включается GA_REQUIRE_PROXY=1; также выкл. при AUTH_DISABLED / API_AUTH_DISABLED.
func proxyGateMW(ba authmw.BearerAuth, enabled bool) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled || loginthrottle.RequestFromTrustedHop(r) || ba.Any(r) {
				next.ServeHTTP(w, r)
				return
			}
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error": "direct backend access denied; use reverse proxy",
			})
		})
	}
}

// withTimeout навешивает жёсткий дедлайн на быстрые read-эндпоинты.
func withTimeout(next http.Handler, d time.Duration) http.Handler {
	return http.TimeoutHandler(next, d, `{"error":"request timeout"}`)
}
