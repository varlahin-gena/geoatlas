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
	writeSessionCookie(w, r, token, cookieMaxAge(ttl))
	SetCSRFCookie(w, r, NewCSRFToken(), ttl)
}

func ClearCookie(w http.ResponseWriter, r *http.Request) {
	writeSessionCookie(w, r, "", -1)
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
	writeCSRFCookie(w, r, token, cookieMaxAge(ttl))
}

func ClearCSRFCookie(w http.ResponseWriter, r *http.Request) {
	writeCSRFCookie(w, r, "", -1)
}

// EnsureCSRFCookie выдаёт CSRF cookie, если его ещё нет (миграция старых сессий).
func EnsureCSRFCookie(w http.ResponseWriter, r *http.Request, ttl time.Duration) {
	if c, err := r.Cookie(auth.CSRFCookieName); err == nil && c != nil && c.Value != "" {
		return
	}
	SetCSRFCookie(w, r, NewCSRFToken(), ttl)
}

func cookieMaxAge(ttl time.Duration) int {
	if ttl > 0 {
		return int(ttl.Seconds())
	}
	return int((12 * time.Hour).Seconds())
}

func writeSessionCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	c := newBaseCookie(auth.CookieName, value, maxAge)
	c.HttpOnly = true
	applyCookieSecure(c, r)
	writeCookie(w, c)
}

func writeCSRFCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	c := newBaseCookie(auth.CSRFCookieName, value, maxAge)
	c.HttpOnly = false // double-submit: frontend reads this into X-CSRF-Token
	applyCookieSecure(c, r)
	writeCookie(w, c)
}

func newBaseCookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
		MaxAge:   maxAge,
	}
}

func applyCookieSecure(c *http.Cookie, r *http.Request) {
	if !cookieSecure(r) {
		c.Secure = false
	}
}

func writeCookie(w http.ResponseWriter, c *http.Cookie) {
	// codeql[go/cookie-secure-not-set]
	// codeql[go/cookie-http-only-not-set]
	http.SetCookie(w, c)
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
