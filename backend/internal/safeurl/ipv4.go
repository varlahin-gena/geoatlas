// Package safeurl — IPv4-only URL checks against SSRF (private/metadata targets).
package safeurl

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"network_monitor/internal/reputation"
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
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() == nil {
			return fmt.Errorf("ipv6 urls are not supported")
		}
		if reputation.IsNonPublicIPv4IP(ip) {
			return fmt.Errorf("url host resolves to a non-public address")
		}
		return nil
	}
	if _, blocked := blockedHostnames[strings.ToLower(host)]; blocked {
		return fmt.Errorf("url host is not allowed")
	}

	ips, err := LookupIPv4(host)
	if err != nil {
		return fmt.Errorf("url host lookup failed")
	}
	if len(ips) == 0 {
		return fmt.Errorf("url host has no ipv4 addresses")
	}
	for _, ip := range ips {
		if reputation.IsNonPublicIPv4IP(ip) {
			return fmt.Errorf("url host resolves to a non-public address")
		}
	}
	return nil
}

// SecureHTTPClient returns a shallow copy of base with SSRF-safe CheckRedirect.
func SecureHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		base = &http.Client{}
	}
	c := *base
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
