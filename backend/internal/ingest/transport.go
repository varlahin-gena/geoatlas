package ingest

import (
	"crypto/subtle"
	"strings"
)

const (
	markerPrefix = "@@nm/"
	markerMid    = "/@@"
)

// ResolveTransport снимает маркер транспорта (+ опциональный shared secret).
// Форматы:
//
//	@@nm/udp/<token>/@@payload
//	@@nm/tcp/<token>/@@payload
//	@@nm/udp/@@payload          — legacy (только если expectedToken пуст / insecure)
//	@@nm/tcp/@@payload
func ResolveTransport(line, fallback string) (transport, payload string) {
	transport, payload, _ = ResolveTransportAuth(line, fallback, "")
	return transport, payload
}

// ResolveTransportAuth как ResolveTransport, но проверяет token.
// ok=false — строку нужно дропнуть (неверный/отсутствующий секрет).
// Если expectedToken пуст — принимаем legacy @@nm/{udp|tcp}/@@ и новый формат с любым token
// (dev / NM_ALLOW_INSECURE); иначе требуется точное совпадение token.
func ResolveTransportAuth(line, fallback, expectedToken string) (transport, payload string, ok bool) {
	if !strings.HasPrefix(line, markerPrefix) {
		return fallback, line, expectedToken == ""
	}
	rest := strings.TrimPrefix(line, markerPrefix)
	// rest: udp/<token>/@@payload  or  udp/@@payload
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		return fallback, line, expectedToken == ""
	}
	tr := rest[:slash]
	if tr != "udp" && tr != "tcp" {
		return fallback, line, expectedToken == ""
	}
	after := rest[slash+1:]
	// legacy: @@payload
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
