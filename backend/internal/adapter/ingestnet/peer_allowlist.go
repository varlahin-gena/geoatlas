package ingestnet

import (
	"net"
	"strings"
	"sync"
	"time"
)

// PeerAllowlist — Accept только с разрешённых peer IP (CIDR / hostname).
type PeerAllowlist struct {
	mu      sync.RWMutex
	nets    []*net.IPNet
	hosts   []string
	cache   map[string]time.Time // host -> last resolve attempt time
	cacheIP map[string][]net.IP
	ttl     time.Duration
}

func NewPeerAllowlist(csv string) *PeerAllowlist {
	a := &PeerAllowlist{
		cache:   make(map[string]time.Time),
		cacheIP: make(map[string][]net.IP),
		ttl:     30 * time.Second,
	}
	for _, raw := range strings.Split(csv, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.Contains(raw, "/") {
			if _, n, err := net.ParseCIDR(raw); err == nil {
				a.nets = append(a.nets, n)
			}
			continue
		}
		if ip := net.ParseIP(raw); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			a.nets = append(a.nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		a.hosts = append(a.hosts, raw)
	}
	return a
}

func (a *PeerAllowlist) Empty() bool {
	return a == nil || (len(a.nets) == 0 && len(a.hosts) == 0)
}

func (a *PeerAllowlist) Allows(addr net.Addr) bool {
	if a == nil || a.Empty() {
		return true // no allowlist configured
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	a.mu.RLock()
	for _, n := range a.nets {
		if n.Contains(ip) {
			a.mu.RUnlock()
			return true
		}
	}
	hosts := append([]string(nil), a.hosts...)
	a.mu.RUnlock()

	for _, name := range hosts {
		for _, resolved := range a.resolve(name) {
			if resolved.Equal(ip) {
				return true
			}
		}
	}
	return false
}

func (a *PeerAllowlist) resolve(name string) []net.IP {
	now := time.Now()
	a.mu.RLock()
	if t, ok := a.cache[name]; ok && now.Sub(t) < a.ttl {
		ips := a.cacheIP[name]
		a.mu.RUnlock()
		return ips
	}
	a.mu.RUnlock()

	addrs, err := net.LookupIP(name)
	if err != nil {
		a.mu.Lock()
		a.cache[name] = now
		a.cacheIP[name] = nil
		a.mu.Unlock()
		return nil
	}
	a.mu.Lock()
	a.cache[name] = now
	a.cacheIP[name] = addrs
	a.mu.Unlock()
	return addrs
}
