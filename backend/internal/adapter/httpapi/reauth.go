package httpapi

import (
	"net/http"
	"strings"

	"geoatlas/internal/adapter/httpapi/authmw"
	"geoatlas/internal/auth"
	"geoatlas/internal/config"
	usecaseauth "geoatlas/internal/usecase/auth"
)

// ReauthChecker — повторная проверка пароля актёра для чувствительных операций cookie-сессии.
// Bearer / AUTH_DISABLED пропускаются (как CSRF).
type ReauthChecker struct {
	cfg      config.Config
	authUC   *usecaseauth.Service
	sessions SessionParser
	ba       authmw.BearerAuth
}

func NewReauthChecker(cfg config.Config, authUC *usecaseauth.Service, sessions SessionParser, apiTokens APITokenStore) ReauthChecker {
	envTokens := cfg.APIAuthTokens()
	if cfg.APIAuthDisabled {
		envTokens = nil
	}
	opsTokens := cfg.APIOpsTokens()
	if cfg.APIAuthDisabled {
		opsTokens = nil
	}
	return ReauthChecker{
		cfg:      cfg,
		authUC:   authUC,
		sessions: sessions,
		ba:       newBearerAuth(envTokens, opsTokens, apiTokens),
	}
}

// Require проверяет current_password для cookie-сессии. Возвращает username актёра.
func (c ReauthChecker) Require(w http.ResponseWriter, r *http.Request, password string) (string, bool) {
	if c.cfg.AuthDisabled {
		return actorFromRequest(r), true
	}
	if c.ba.Any(r) {
		return actorFromRequest(r), true
	}
	password = strings.TrimSpace(password)
	if password == "" {
		writeBadRequest(w, "current_password is required")
		return "", false
	}
	sess, ok := actorSession(r, c.sessions)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return "", false
	}
	if c.authUC == nil || !c.authUC.VerifyPassword(sess.Username, password) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "current password is incorrect"})
		return "", false
	}
	return sess.Username, true
}

func actorSession(r *http.Request, sessions SessionParser) (auth.Session, bool) {
	if sess, ok := SessionFromContext(r.Context()); ok {
		return sess, true
	}
	if sessions == nil {
		return auth.Session{}, false
	}
	sess, err := SessionFromRequest(r, sessions)
	return sess, err == nil
}
