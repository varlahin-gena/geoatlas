package reputation

import (
	"encoding/binary"
	"math/bits"
	"net"
)

// FormatNetworkPreferCIDR — CIDR, если диапазон ровно префикс; иначе a-b или single IP.
func FormatNetworkPreferCIDR(start, end uint32) string {
	if start == end {
		return Uint32ToIP(start)
	}
	if prefix, ok := exactPrefixLen(start, end); ok {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, start)
		return ip.String() + "/" + itoa(prefix)
	}
	return FormatNetwork(start, end)
}

func exactPrefixLen(start, end uint32) (int, bool) {
	if start > end {
		return 0, false
	}
	span := end - start + 1
	// span должен быть степенью 2
	if span == 0 || (span&(span-1)) != 0 {
		return 0, false
	}
	prefix := 32 - bits.TrailingZeros32(span)
	mask := uint32(0xFFFFFFFF) << (32 - prefix)
	if start&mask != start {
		return 0, false
	}
	if start|(^mask) != end {
		return 0, false
	}
	return prefix, true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [2]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
