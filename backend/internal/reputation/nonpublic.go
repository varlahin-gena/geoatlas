package reputation

import "net"

// IsNonPublicIPv4IP — частные/спец. адреса, для которых репутация не имеет смысла (hot path Lookup).
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
