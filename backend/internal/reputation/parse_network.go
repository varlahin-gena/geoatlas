package reputation

import (
	"net"
	"strings"

	"network_monitor/internal/geoip"
)

// ParseNetworkField — CIDR / a-b / single IPv4 (тот же контракт, что geoip).
func ParseNetworkField(network string) (uint32, uint32, bool) {
	return geoip.ParseNetworkField(network)
}

// FormatNetwork форматирует диапазон как CIDR или single IP.
func FormatNetwork(start, end uint32) string {
	return geoip.FormatNetwork(start, end)
}

// IPToUint32 парсит IPv4 в uint32; 0 если невалидно.
func IPToUint32(ipStr string) uint32 {
	return geoip.IPToUint32(ipStr)
}

// Uint32ToIP — dotted-quad.
func Uint32ToIP(v uint32) string {
	return geoip.Uint32ToIP(v)
}

func parseIPv4Line(line string) (uint32, uint32, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return 0, 0, false
	}
	// FireHOL иногда даёт "IP # comment"
	if i := strings.IndexByte(line, '#'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if line == "" {
		return 0, 0, false
	}
	// Пропуск явного IPv6
	if strings.Contains(line, ":") {
		return 0, 0, false
	}
	start, end, ok := ParseNetworkField(line)
	if !ok {
		return 0, 0, false
	}
	// sanity: должен быть валидный IPv4
	if net.ParseIP(Uint32ToIP(start)) == nil {
		return 0, 0, false
	}
	return start, end, true
}
