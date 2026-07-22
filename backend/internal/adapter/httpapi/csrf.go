package httpapi

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	"network_monitor/internal/auth"
)

// csrfMW — double-submit CSRF для cookie-сессий на небезопасных методах.
// Bearer / AUTH_DISABLED пропускаются. GET/HEAD/OPTIONS — без проверки.
func csrfMW(apiToken string, authDisabled bool) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authDisabled || safeMethod(r.Method) || bearerOK(r, apiToken) {
				next.ServeHTTP(w, r)
				return
			}
			// Нет session cookie — auth MW отклонит; CSRF не применяем.
			if c, err := r.Cookie(auth.CookieName); err != nil || c == nil || c.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !csrfOriginOK(r) {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "csrf origin rejected"})
				return
			}
			cookie, err := r.Cookie(auth.CSRFCookieName)
			header := strings.TrimSpace(r.Header.Get(auth.CSRFHeaderName))
			if err != nil || cookie == nil || cookie.Value == "" || header == "" ||
				subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "csrf token missing or invalid"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func safeMethod(m string) bool {
	switch strings.ToUpper(m) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

// csrfOriginOK: при наличии Origin/Referer — только same-origin (или пустой Host-less).
// Запросы без Origin/Referer (curl, scripts) проходят при валидном CSRF header.
func csrfOriginOK(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" {
		return originHostMatches(origin, r.Host)
	}
	ref := strings.TrimSpace(r.Header.Get("Referer"))
	if ref == "" {
		return true
	}
	return originHostMatches(ref, r.Host)
}

func originHostMatches(raw, host string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, host)
}
