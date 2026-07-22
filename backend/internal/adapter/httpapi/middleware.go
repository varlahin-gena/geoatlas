package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"network_monitor/internal/auth"
	"network_monitor/internal/metrics"
)

type ctxKey int

const (
	sessionCtxKey   ctxKey = 1
	requestIDCtxKey ctxKey = 2
)

const requestIDHeader = "X-Request-ID"

func SessionFromContext(ctx context.Context) (auth.Session, bool) {
	if ctx == nil {
		return auth.Session{}, false
	}
	s, ok := ctx.Value(sessionCtxKey).(auth.Session)
	return s, ok
}

func withSession(ctx context.Context, s auth.Session) context.Context {
	return context.WithValue(ctx, sessionCtxKey, s)
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

// routeLabel возвращает path template mux (низкая cardinality), иначе "unmatched".
func routeLabel(r *http.Request) string {
	if route := mux.CurrentRoute(r); route != nil {
		if tpl, err := route.GetPathTemplate(); err == nil && tpl != "" {
			return tpl
		}
	}
	return "unmatched"
}

// loggingMW пишет строку доступа и latency в Prometheus.
func loggingMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start)
		path := r.URL.Path
		route := routeLabel(r)
		slog.Info("http",
			"request_id", RequestIDFromContext(r.Context()),
			"method", r.Method,
			"path", path,
			"route", route,
			"status", rec.status,
			"bytes", rec.bytes,
			"duration", elapsed.Round(time.Millisecond).String(),
		)
		if route != "/metrics" {
			metrics.APIRequestDuration.WithLabelValues(
				r.Method,
				route,
				strconv.Itoa(rec.status),
			).Observe(elapsed.Seconds())
		}
	})
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

// authMW проверяет Bearer-токен ИЛИ валидную сессию пользователя.
// apiAuthDisabled / authDisabled — доступ открыт (dev).
func authMW(token string, sessions *auth.SessionManager, users *auth.UserStore, apiAuthDisabled, authDisabled bool) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authDisabled || apiAuthDisabled {
				next.ServeHTTP(w, r)
				return
			}
			if bearerOK(r, token) {
				next.ServeHTTP(w, r)
				return
			}
			if sess, err := SessionFromRequest(r, sessions); err == nil {
				live, ok := auth.LiveSession(users, sess)
				if !ok {
					writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
					return
				}
				if denyIfMustReset(w, users, live.Username) {
					return
				}
				next.ServeHTTP(w, r.WithContext(withSession(r.Context(), live)))
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		})
	}
}

// requireLoginMW требует cookie-сессию, Bearer API-токен, либо AUTH_DISABLED.
func requireLoginMW(token string, sessions *auth.SessionManager, users *auth.UserStore, authDisabled bool) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authDisabled {
				next.ServeHTTP(w, r)
				return
			}
			if bearerOK(r, token) {
				next.ServeHTTP(w, r)
				return
			}
			sess, err := SessionFromRequest(r, sessions)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			live, ok := auth.LiveSession(users, sess)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			if denyIfMustReset(w, users, live.Username) {
				return
			}
			next.ServeHTTP(w, r.WithContext(withSession(r.Context(), live)))
		})
	}
}

// requireAdminMW — роль administrator (из UserStore, не из cookie), Bearer API-токен, либо AUTH_DISABLED.
func requireAdminMW(token string, sessions *auth.SessionManager, users *auth.UserStore, authDisabled bool) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authDisabled {
				next.ServeHTTP(w, r)
				return
			}
			if bearerOK(r, token) {
				next.ServeHTTP(w, r)
				return
			}
			sess, err := SessionFromRequest(r, sessions)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			live, ok := auth.LiveSession(users, sess)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			if !auth.IsAdmin(live.Role) {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
				return
			}
			if denyIfMustReset(w, users, live.Username) {
				return
			}
			next.ServeHTTP(w, r.WithContext(withSession(r.Context(), live)))
		})
	}
}

// requireOpsMW — mutate / metrics / ingest stats: Bearer или administrator.
// API_AUTH_DISABLED или AUTH_DISABLED открывают доступ (dev / локальный контур).
func requireOpsMW(token string, sessions *auth.SessionManager, users *auth.UserStore, apiAuthDisabled, authDisabled bool) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authDisabled || apiAuthDisabled {
				next.ServeHTTP(w, r)
				return
			}
			if bearerOK(r, token) {
				next.ServeHTTP(w, r)
				return
			}
			sess, err := SessionFromRequest(r, sessions)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			live, ok := auth.LiveSession(users, sess)
			if !ok {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			if !auth.IsAdmin(live.Role) {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
				return
			}
			if denyIfMustReset(w, users, live.Username) {
				return
			}
			next.ServeHTTP(w, r.WithContext(withSession(r.Context(), live)))
		})
	}
}

func denyIfMustReset(w http.ResponseWriter, users *auth.UserStore, username string) bool {
	if users != nil && users.MustReset(username) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "password reset required"})
		return true
	}
	return false
}

func bearerOK(r *http.Request, token string) bool {
	if token == "" || r == nil {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

// withTimeout навешивает жёсткий дедлайн на быстрые read-эндпоинты.
// TimeoutHandler проставляет deadline в request-контекст, поэтому
// ClickHouse-запросы, использующие r.Context(), тоже отменяются.
func withTimeout(next http.Handler, d time.Duration) http.Handler {
	return http.TimeoutHandler(next, d, `{"error":"request timeout"}`)
}
