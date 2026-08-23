// Package safeurl — IPv4-only URL checks against SSRF (private/metadata targets).
package safeurl

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"geoatlas/internal/netutil"
)

const MaxRedirects = 3

var blockedHostnames = map[string]struct{}{
	"metadata.google.internal": {},
	"metadata":                 {},
	"kubernetes.default":       {},
	"kubernetes.default.svc":   {},
}

// LookupIPv4 resolves host to IPv4 addresses (injectable in tests).
var LookupIPv4 = func(host string) ([]net.IP, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	out := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil {
			out = append(out, ip4)
		}
	}
	return out, nil
}

// DialContext connects only to validated public IPv4 (injectable in tests).
var DialContext = defaultDialContext

// ValidateHTTPURL checks scheme/host and that all resolved IPv4 are public.
// IPv6 literals and hosts without A records are rejected (product is IPv4-only).
func ValidateHTTPURL(raw string) error {
	u, err := parseHTTPURL(raw)
	if err != nil {
		return err
	}
	return validateParsed(u)
}

func parseHTTPURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("url is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("url must be http(s)")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("url host is required")
	}
	if u.User != nil {
		return nil, fmt.Errorf("url must not contain userinfo")
	}
	return u, nil
}

func validateParsed(u *url.URL) error {
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("url host is required")
	}
	_, err := resolvePublicIPv4(host)
	return err
}

// resolvePublicIPv4 returns dialable public IPv4s for host (literal or DNS).
func resolvePublicIPv4(host string) ([]net.IP, error) {
	if host == "" {
		return nil, fmt.Errorf("url host is required")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() == nil {
			return nil, fmt.Errorf("ipv6 urls are not supported")
		}
		if netutil.IsNonPublicIPv4IP(ip) {
			return nil, fmt.Errorf("url host resolves to a non-public address")
		}
		return []net.IP{ip.To4()}, nil
	}
	if _, blocked := blockedHostnames[strings.ToLower(host)]; blocked {
		return nil, fmt.Errorf("url host is not allowed")
	}

	ips, err := LookupIPv4(host)
	if err != nil {
		return nil, fmt.Errorf("url host lookup failed")
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("url host has no ipv4 addresses")
	}
	out := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		ip4 := ip.To4()
		if ip4 == nil {
			continue
		}
		if netutil.IsNonPublicIPv4IP(ip4) {
			return nil, fmt.Errorf("url host resolves to a non-public address")
		}
		out = append(out, ip4)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("url host has no ipv4 addresses")
	}
	return out, nil
}

func defaultDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := resolvePublicIPv4(host)
	if err != nil {
		return nil, err
	}
	d := net.Dialer{Timeout: 30 * time.Second}
	var last error
	for _, ip := range ips {
		var dialErr error
		conn, dialErr := d.DialContext(ctx, "tcp4", net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		last = dialErr
	}
	if last == nil {
		return nil, fmt.Errorf("no public ipv4 to dial")
	}
	return nil, last
}

func cloneTransport(base http.RoundTripper) *http.Transport {
	if base == nil {
		return http.DefaultTransport.(*http.Transport).Clone()
	}
	if ht, ok := base.(*http.Transport); ok {
		return ht.Clone()
	}
	return http.DefaultTransport.(*http.Transport).Clone()
}

// SecureHTTPClient returns a shallow copy of base with SSRF-safe dial + redirects.
// Dial uses tcp4 only to addresses that resolve to public IPv4 at connect time
// (mitigates DNS rebinding and Happy-Eyeballs AAAA to private/ULA).
func SecureHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		base = &http.Client{}
	}
	c := *base
	tr := cloneTransport(base.Transport)
	tr.DialContext = DialContext
	c.Transport = tr
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= MaxRedirects {
			return fmt.Errorf("too many redirects")
		}
		if req.URL == nil {
			return fmt.Errorf("redirect missing url")
		}
		if err := validateParsed(req.URL); err != nil {
			return err
		}
		return nil
	}
	return &c
}
