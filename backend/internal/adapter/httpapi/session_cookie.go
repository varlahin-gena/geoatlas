package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"network_monitor/internal/auth"
)

// SetCookie пишет session + CSRF cookies.
func SetCookie(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) {
	http.SetCookie(w, appCookie(auth.CookieName, token, true, cookieMaxAge(ttl)))
	SetCSRFCookie(w, r, NewCSRFToken(), ttl)
}

func ClearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, appCookie(auth.CookieName, "", true, -1))
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
	http.SetCookie(w, appCookie(auth.CSRFCookieName, token, false, cookieMaxAge(ttl)))
}

func ClearCSRFCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, appCookie(auth.CSRFCookieName, "", false, -1))
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

// appCookie — session/CSRF cookie. Secure всегда true: CodeQL go/cookie-secure-not-set
// требует константу; браузеры всё равно отправляют Secure-cookie на localhost/127.0.0.1.
func appCookie(name, value string, httpOnly bool, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: httpOnly,
		SameSite: http.SameSiteStrictMode,
		Secure:   true,
		MaxAge:   maxAge,
	}
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
