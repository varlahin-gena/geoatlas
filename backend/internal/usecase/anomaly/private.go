package anomaly

import (
	"net"
	"strings"
)

func ipInNets(ip string, nets []IPRange) bool {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return false
	}
	v4 := parsed.To4()
	if v4 == nil {
		return false
	}
	u := uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3])
	for _, n := range nets {
		if n.End < n.Start {
			continue
		}
		if u >= n.Start && u <= n.End {
			return true
		}
	}
	return false
}

func isPrivateOrLocal(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsLinkLocalUnicast()
}
