package threatprot

import (
	"net/http"
	"strings"
)

// SuspiciousRequest reports path traversal, control chars, or header injection patterns
// (Apigee RegularExpressionProtection: URI path + header CRLF).
func SuspiciousRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if suspiciousPath(r.URL.Path) {
		return true
	}
	if r.URL.RawQuery != "" && suspiciousQuery(r.URL.RawQuery) {
		return true
	}
	for name, vals := range r.Header {
		if suspiciousHeaderName(name) {
			return true
		}
		for _, v := range vals {
			if strings.ContainsAny(v, "\r\n\x00") {
				return true
			}
		}
	}
	return false
}

func suspiciousPath(path string) bool {
	if path == "" {
		return false
	}
	if strings.Contains(path, "\x00") {
		return true
	}
	lower := strings.ToLower(path)
	if strings.Contains(lower, "/../") || strings.Contains(lower, "/..") {
		return true
	}
	if strings.HasPrefix(lower, "../") || strings.Contains(lower, "%2e%2e") {
		return true
	}
	return false
}

func suspiciousQuery(raw string) bool {
	if strings.Contains(raw, "\x00") || strings.ContainsAny(raw, "\r\n") {
		return true
	}
	lower := strings.ToLower(raw)
	return strings.Contains(lower, "%00") || strings.Contains(lower, "%0d") || strings.Contains(lower, "%0a")
}

func suspiciousHeaderName(name string) bool {
	return strings.ContainsAny(name, "\r\n\x00")
}
