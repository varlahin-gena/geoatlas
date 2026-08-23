package authmw

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"geoatlas/internal/auth"
)

type ctxKey int

const sessionCtxKey ctxKey = 1

// SessionFromContext returns the live session attached by Require* middleware.
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

// SessionLoader loads a cookie session (httpapi.SessionFromRequest).
type SessionLoader func(r *http.Request) (auth.Session, error)

// UserDirectory — минимальный порт для LiveSession / MustReset.
type UserDirectory interface {
	Get(string) (auth.UserPublic, bool)
	SessionVersion(string) (int64, bool)
	MustReset(string) bool
}

// TokenStore — named API token verify.
type TokenStore interface {
	Verify(string) (string, bool)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("authmw writeJSON: marshal failed", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func denyIfMustReset(w http.ResponseWriter, users UserDirectory, username string) bool {
	if users != nil && users.MustReset(username) {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "password reset required"})
		return true
	}
	return false
}

// Middleware matches httpapi middleware signature.
type Middleware func(http.Handler) http.Handler

// RequireLogin требует cookie-сессию, Bearer (scope≥read), либо AUTH_DISABLED.
func RequireLogin(ba BearerAuth, load SessionLoader, users UserDirectory, authDisabled bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authDisabled {
				next.ServeHTTP(w, r)
				return
			}
			if ba.OK(r, auth.ScopeRead) {
				next.ServeHTTP(w, r)
				return
			}
			if load == nil {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			sess, err := load(r)
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

// RequireAdmin — роль administrator, Bearer scope=admin, либо AUTH_DISABLED.
func RequireAdmin(ba BearerAuth, load SessionLoader, users UserDirectory, authDisabled bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authDisabled {
				next.ServeHTTP(w, r)
				return
			}
			if ba.OK(r, auth.ScopeAdmin) {
				next.ServeHTTP(w, r)
				return
			}
			if load == nil {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			sess, err := load(r)
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

// RequireOps — Bearer scope≥ops или administrator.
func RequireOps(ba BearerAuth, load SessionLoader, users UserDirectory, apiAuthDisabled, authDisabled bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authDisabled || apiAuthDisabled {
				next.ServeHTTP(w, r)
				return
			}
			if ba.OK(r, auth.ScopeOps) {
				next.ServeHTTP(w, r)
				return
			}
			if load == nil {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			sess, err := load(r)
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
