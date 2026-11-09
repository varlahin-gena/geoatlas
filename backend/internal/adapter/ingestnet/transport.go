package ingestnet

import (
	"crypto/subtle"
	"strings"
)

const (
	markerPrefix       = "@@ga/"
	legacyMarkerPrefix = "@@nm/"
	markerMid          = "/@@"
	defaultConnectorID = "default"
)

// ResolveTransport снимает маркер транспорта (+ опциональный shared secret).
// Форматы:
//
//	@@ga/udp/<token>/<connector_id>/@@payload
//	@@ga/tcp/<token>/<connector_id>/@@payload
//	@@ga/udp/<token>/@@payload          — connector_id=default (compat)
//	@@ga/tcp/<token>/@@payload
//	@@ga/udp/@@payload                  — без token (только insecure)
//	@@ga/tcp/@@payload
//
// Plain lines with fallback "http" skip the shared secret (API already auth'd).
func ResolveTransport(line, fallback string) (transport, payload string) {
	transport, payload, _, _ = ResolveTransportAuth(line, fallback, "")
	return transport, payload
}

// ResolveTransportAuth как ResolveTransport, но проверяет token и возвращает connectorID.
// ok=false — строку нужно дропнуть (неверный/отсутствующий секрет).
// Если expectedToken пуст — принимаем @@ga/{udp|tcp}/@@ и формат с любым token
// (dev / GA_ALLOW_INSECURE); иначе требуется точное совпадение token.
// Plain (no marker) lines with fallback "http" always ok — secret is for :1514 only.
// connectorID: "default" for legacy/syslog markers without id; "" for http ingest.
func ResolveTransportAuth(line, fallback, expectedToken string) (transport, payload, connectorID string, ok bool) {
	if strings.HasPrefix(line, legacyMarkerPrefix) {
		return "", "", "", false
	}
	if !strings.HasPrefix(line, markerPrefix) {
		if fallback == "http" {
			return "http", line, "", true
		}
		return fallback, line, defaultConnectorID, expectedToken == ""
	}
	rest := strings.TrimPrefix(line, markerPrefix)
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		return fallback, line, defaultConnectorID, expectedToken == ""
	}
	tr := rest[:slash]
	if tr != "udp" && tr != "tcp" {
		return fallback, line, defaultConnectorID, expectedToken == ""
	}
	after := rest[slash+1:]
	if strings.HasPrefix(after, "@@") {
		if expectedToken != "" {
			return tr, "", "", false
		}
		return tr, strings.TrimPrefix(after, "@@"), defaultConnectorID, true
	}
	mid := strings.Index(after, markerMid)
	if mid < 0 {
		return fallback, line, defaultConnectorID, expectedToken == ""
	}
	tokenPart := after[:mid]
	payload = after[mid+len(markerMid):]

	token, connectorID := splitTokenConnector(tokenPart)
	if connectorID == "" {
		connectorID = defaultConnectorID
	}

	if expectedToken == "" {
		return tr, payload, connectorID, true
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		return tr, "", "", false
	}
	return tr, payload, connectorID, true
}

// splitTokenConnector: "token" or "token/connector_id".
func splitTokenConnector(part string) (token, connectorID string) {
	if i := strings.IndexByte(part, '/'); i >= 0 {
		return part[:i], part[i+1:]
	}
	return part, ""
}
