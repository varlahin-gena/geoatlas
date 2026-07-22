package ingest

import "strings"

const (
	transportPrefixUDP = "@@nm/udp/@@"
	transportPrefixTCP = "@@nm/tcp/@@"
)

// ResolveTransport снимает маркер транспорта, проставленный syslog-ng на входе 514/udp|tcp.
// Если маркера нет, используется fallback (транспорт listener'а backend).
func ResolveTransport(line, fallback string) (transport, payload string) {
	switch {
	case strings.HasPrefix(line, transportPrefixUDP):
		return "udp", strings.TrimPrefix(line, transportPrefixUDP)
	case strings.HasPrefix(line, transportPrefixTCP):
		return "tcp", strings.TrimPrefix(line, transportPrefixTCP)
	default:
		return fallback, line
	}
}
