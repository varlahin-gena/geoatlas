package ingestnet

import (
	"crypto/subtle"
	"strings"
)

const (
	markerPrefix       = "@@ga/"
	legacyMarkerPrefix = "@@nm/"
	markerMid          = "/@@"
)

// ResolveTransport снимает маркер транспорта (+ опциональный shared secret).
// Форматы:
//
//	@@ga/udp/<token>/@@payload
//	@@ga/tcp/<token>/@@payload
//	@@ga/udp/@@payload          — без token (только если expectedToken пуст / insecure)
//	@@ga/tcp/@@payload
//
// Plain lines with fallback "http" skip the shared secret (API already auth'd).
func ResolveTransport(line, fallback string) (transport, payload string) {
	transport, payload, _ = ResolveTransportAuth(line, fallback, "")
	return transport, payload
}

// ResolveTransportAuth как ResolveTransport, но проверяет token.
// ok=false — строку нужно дропнуть (неверный/отсутствующий секрет).
// Если expectedToken пуст — принимаем @@ga/{udp|tcp}/@@ и формат с любым token
// (dev / GA_ALLOW_INSECURE); иначе требуется точное совпадение token.
// Plain (no marker) lines with fallback "http" always ok — secret is for :1514 only.
func ResolveTransportAuth(line, fallback, expectedToken string) (transport, payload string, ok bool) {
	if strings.HasPrefix(line, legacyMarkerPrefix) {
		return "", "", false
	}
	if !strings.HasPrefix(line, markerPrefix) {
		if fallback == "http" {
			return "http", line, true
		}
		return fallback, line, expectedToken == ""
	}
	rest := strings.TrimPrefix(line, markerPrefix)
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		return fallback, line, expectedToken == ""
	}
	tr := rest[:slash]
	if tr != "udp" && tr != "tcp" {
		return fallback, line, expectedToken == ""
	}
	after := rest[slash+1:]
	if strings.HasPrefix(after, "@@") {
		if expectedToken != "" {
			return tr, "", false
		}
		return tr, strings.TrimPrefix(after, "@@"), true
	}
	mid := strings.Index(after, markerMid)
	if mid < 0 {
		return fallback, line, expectedToken == ""
	}
	token := after[:mid]
	payload = after[mid+len(markerMid):]
	if expectedToken == "" {
		return tr, payload, true
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		return tr, "", false
	}
	return tr, payload, true
}
