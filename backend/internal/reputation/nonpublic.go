package reputation

import "net"

// IsNonPublicIPv4 — частные/спец. адреса, для которых репутация не имеет смысла.
// RFC1918, loopback, link-local, multicast, unspecified, CGNAT 100.64/10.
func IsNonPublicIPv4(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true
	}
	return IsNonPublicIPv4IP(ip)
}

// IsNonPublicIPv4IP — тот же тест без повторного ParseIP (hot path Lookup).
func IsNonPublicIPv4IP(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return true
	}
	if ip4.IsLoopback() || ip4.IsLinkLocalUnicast() || ip4.IsLinkLocalMulticast() ||
		ip4.IsMulticast() || ip4.IsUnspecified() || ip4.IsPrivate() {
		return true
	}
	// CGNAT / shared address space (RFC 6598)
	if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return true
	}
	return false
}
