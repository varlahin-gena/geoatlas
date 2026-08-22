package anomaly

import (
	"fmt"
	"net"
	"strings"
)

// MapLinkFor rebuilds the map view from persisted anomaly fields.
func MapLinkFor(e Event) MapLink {
	switch e.Code {
	case CodePortScan, CodeHorizontalScan:
		if src := strings.TrimSpace(e.SrcIP); src != "" {
			return MapLink{Period: "15m", Group: "ip", Filter: "all", Query: "src:" + src}
		}
	case CodeRepNewDst:
		src := strings.TrimSpace(e.SrcIP)
		dst := strings.TrimSpace(e.DstIP)
		q := ""
		if src != "" && dst != "" {
			q = "src:" + src + " dst:" + dst
		}
		return MapLink{Period: "1h", Group: "ip", Filter: "all", Query: q}
	case CodeNewCountryDst:
		if country := strings.TrimSpace(e.DstCountry); country != "" {
			return MapLink{Period: "1h", Group: "country", Filter: "all", Query: "dst:" + country, Country: country}
		}
	case CodeBlockedSurge:
		if link := blockedSurgeMapLink(e); link.Query != "" {
			return link
		}
	}
	return fallbackMapLink(e)
}

func blockedSurgeMapLink(e Event) MapLink {
	q := blockedSurgeQuery(e)
	if q == "" {
		return MapLink{}
	}
	group := "ip"
	query := q
	if city := blockedSurgeCityLabel(e); city != "" && blockedSurgeIsWide(e) {
		group = "city"
		query = "city:" + city
	}
	return MapLink{Period: "1h", Group: group, Filter: "blocked", Query: query}
}

func blockedSurgeCityLabel(e Event) string {
	label := ""
	if e.Detail != nil {
		if raw, ok := e.Detail["label"].(string); ok {
			label = strings.TrimSpace(raw)
		}
	}
	if label == "" {
		return ""
	}
	if i := strings.Index(label, ","); i > 0 {
		return strings.TrimSpace(label[:i])
	}
	return label
}

func blockedSurgeIsWide(e Event) bool {
	network := strings.TrimSpace(e.Device)
	if network == "" && e.Detail != nil {
		if raw, ok := e.Detail["network"].(string); ok {
			network = strings.TrimSpace(raw)
		}
	}
	if network == "" {
		return false
	}
	if strings.Contains(network, "-") {
		return true
	}
	if strings.Contains(network, "/") {
		_, ipNet, err := net.ParseCIDR(network)
		if err != nil {
			return false
		}
		ones, _ := ipNet.Mask.Size()
		return ones <= 16
	}
	prefix := networkPrefixQuery(network)
	parts := strings.Split(strings.TrimSuffix(prefix, "."), ".")
	return len(parts) <= 2
}

func blockedSurgeQuery(e Event) string {
	network := strings.TrimSpace(e.Device)
	if network == "" && e.Detail != nil {
		if raw, ok := e.Detail["network"].(string); ok {
			network = strings.TrimSpace(raw)
		}
	}
	prefix := networkPrefixQuery(network)
	if prefix == "" {
		return ""
	}
	return fmt.Sprintf("(src:%s OR dst:%s)", prefix, prefix)
}

func fallbackMapLink(e Event) MapLink {
	if src := strings.TrimSpace(e.SrcIP); src != "" {
		return MapLink{Period: "1h", Group: "ip", Filter: "all", Query: "src:" + src}
	}
	if dst := strings.TrimSpace(e.DstIP); dst != "" {
		return MapLink{Period: "1h", Group: "ip", Filter: "all", Query: "dst:" + dst}
	}
	if city := firstNonEmpty(e.SrcCity, e.DstCity); city != "" {
		return MapLink{Period: "1h", Group: "city", Filter: "all", Query: "city:" + city}
	}
	if country := firstNonEmpty(e.DstCountry, e.SrcCountry); country != "" {
		return MapLink{Period: "1h", Group: "country", Filter: "all", Query: "country:" + country, Country: country}
	}
	return MapLink{Period: "1h", Group: "city", Filter: "all"}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func networkPrefixQuery(network string) string {
	network = strings.TrimSpace(network)
	if network == "" {
		return ""
	}
	if strings.Contains(network, "/") {
		_, ipNet, err := net.ParseCIDR(network)
		if err != nil {
			return ""
		}
		ip4 := ipNet.IP.To4()
		if ip4 == nil || len(ipNet.Mask) != 4 {
			return ""
		}
		start := uint32(ip4[0])<<24 | uint32(ip4[1])<<16 | uint32(ip4[2])<<8 | uint32(ip4[3])
		mask := uint32(ipNet.Mask[0])<<24 | uint32(ipNet.Mask[1])<<16 | uint32(ipNet.Mask[2])<<8 | uint32(ipNet.Mask[3])
		return prefixFromRange(start, start|^mask)
	}
	if strings.Contains(network, "-") {
		parts := strings.SplitN(network, "-", 2)
		a := net.ParseIP(strings.TrimSpace(parts[0])).To4()
		b := net.ParseIP(strings.TrimSpace(parts[1])).To4()
		if a == nil || b == nil {
			return ""
		}
		start := uint32(a[0])<<24 | uint32(a[1])<<16 | uint32(a[2])<<8 | uint32(a[3])
		end := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		if start > end {
			start, end = end, start
		}
		return prefixFromRange(start, end)
	}
	if ip := net.ParseIP(network).To4(); ip != nil {
		return network
	}
	return ""
}

func prefixFromRange(start, end uint32) string {
	if end < start {
		return ""
	}
	octet := func(v uint32, shift uint) uint32 { return (v >> shift) & 0xFF }
	var parts []string
	for _, shift := range []uint{24, 16, 8, 0} {
		a := octet(start, shift)
		b := octet(end, shift)
		if a != b {
			break
		}
		parts = append(parts, fmt.Sprintf("%d", a))
	}
	switch len(parts) {
	case 0:
		return ""
	case 1, 2, 3:
		return strings.Join(parts, ".") + "."
	default:
		return strings.Join(parts, ".")
	}
}
