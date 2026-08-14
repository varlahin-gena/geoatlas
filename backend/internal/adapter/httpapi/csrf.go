package httpapi

import (
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"network_monitor/internal/auth"
)

// csrfMW — double-submit CSRF для cookie-сессий на небезопасных методах.
// Bearer (любой scope) / AUTH_DISABLED пропускаются. GET/HEAD/OPTIONS — без проверки.
func csrfMW(ba bearerAuth, authDisabled bool) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authDisabled || safeMethod(r.Method) || ba.any(r) {
				next.ServeHTTP(w, r)
				return
			}
			// Нет session cookie — auth MW отклонит; CSRF не применяем.
			if c, err := r.Cookie(auth.CookieName); err != nil || c == nil || c.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !csrfOriginOK(r) {
				slog.Warn("csrf origin rejected",
					"origin", r.Header.Get("Origin"),
					"referer", r.Header.Get("Referer"),
					"host", r.Host,
					"x_forwarded_host", r.Header.Get("X-Forwarded-Host"),
					"path", r.URL.Path,
				)
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

// csrfOriginOK: при наличии Origin/Referer — same-origin (hostname; порт может отличаться
// из‑за nginx $host). Запросы без Origin/Referer (curl) проходят при валидном CSRF header.
//
// Дополнительно: Origin с литеральным IP (типичный self-hosted UI :8080) принимаем —
// Host за reverse-proxy часто backend:8080 / без порта; cookie SameSite=Strict уже
// не пускает cross-site session, double-submit токен всё равно обязателен.
func csrfOriginOK(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" {
		return originAllowed(origin, r)
	}
	ref := strings.TrimSpace(r.Header.Get("Referer"))
	if ref == "" {
		return true
	}
	return originAllowed(ref, r)
}

func originAllowed(raw string, r *http.Request) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	for _, candidate := range csrfHostCandidates(r) {
		if csrfHostsEqual(u.Host, candidate) {
			return true
		}
	}
	// Self-hosted по IP: Origin с literal IP только если hostname совпадает с Host/XFH.
	if hostIsLiteralIP(u.Hostname()) {
		for _, candidate := range csrfHostCandidates(r) {
			ch, _ := splitHostPortLoose(candidate)
			if strings.EqualFold(u.Hostname(), strings.Trim(ch, "[]")) {
				return true
			}
		}
		return false
	}
	return false
}

func csrfHostCandidates(r *http.Request) []string {
	seen := make(map[string]struct{}, 4)
	out := make([]string, 0, 4)
	add := func(s string) {
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			key := strings.ToLower(part)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, part)
		}
	}
	add(r.Host)
	add(r.Header.Get("X-Forwarded-Host"))
	add(r.Header.Get("X-Original-Host"))
	return out
}

func hostIsLiteralIP(host string) bool {
	host = strings.Trim(host, "[]")
	return net.ParseIP(host) != nil
}

// csrfHostsEqual — same-origin по hostname; порт может отсутствовать на одной стороне
// (nginx proxy_set_header Host $host без :8080, а Origin от браузера с портом).
func csrfHostsEqual(a, b string) bool {
	if strings.EqualFold(a, b) {
		return true
	}
	ah, ap := splitHostPortLoose(a)
	bh, bp := splitHostPortLoose(b)
	if !strings.EqualFold(ah, bh) {
		return false
	}
	if ap == "" || bp == "" || ap == bp {
		return true
	}
	return false
}

func splitHostPortLoose(hostport string) (host, port string) {
	h, p, err := net.SplitHostPort(hostport)
	if err == nil {
		return h, p
	}
	return strings.Trim(hostport, "[]"), ""
}
