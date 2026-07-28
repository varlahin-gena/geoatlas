package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"network_monitor/internal/auth"
)

// SetCookie пишет session + CSRF cookies.
func SetCookie(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) {
	maxAge := int((12 * time.Hour).Seconds())
	if ttl > 0 {
		maxAge = int(ttl.Seconds())
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   cookieSecure(r),
		MaxAge:   maxAge,
	})
	SetCSRFCookie(w, r, NewCSRFToken(), ttl)
}

func ClearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   cookieSecure(r),
		MaxAge:   -1,
	})
	ClearCSRFCookie(w, r)
}

// NewCSRFToken — случайный токен для double-submit cookie.
func NewCSRFToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(b[:])
}

// SetCSRFCookie пишет читаемый JS cookie (не HttpOnly) для X-CSRF-Token.
func SetCSRFCookie(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) {
	if token == "" {
		token = NewCSRFToken()
	}
	maxAge := int((12 * time.Hour).Seconds())
	if ttl > 0 {
		maxAge = int(ttl.Seconds())
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CSRFCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Secure:   cookieSecure(r),
		MaxAge:   maxAge,
	})
}

func ClearCSRFCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CSRFCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteStrictMode,
		Secure:   cookieSecure(r),
		MaxAge:   -1,
	})
}

// EnsureCSRFCookie выдаёт CSRF cookie, если его ещё нет (миграция старых сессий).
func EnsureCSRFCookie(w http.ResponseWriter, r *http.Request, ttl time.Duration) {
	if c, err := r.Cookie(auth.CSRFCookieName); err == nil && c != nil && c.Value != "" {
		return
	}
	SetCSRFCookie(w, r, NewCSRFToken(), ttl)
}

func cookieSecure(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func SessionFromRequest(r *http.Request, m SessionParser) (auth.Session, error) {
	if m == nil || r == nil {
		return auth.Session{}, auth.ErrInvalidSession
	}
	c, err := r.Cookie(auth.CookieName)
	if err != nil || c == nil || c.Value == "" {
		return auth.Session{}, auth.ErrInvalidSession
	}
	return m.Parse(c.Value)
}
