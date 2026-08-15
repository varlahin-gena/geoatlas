package ingestnet

import (
	"net"
	"testing"
)

type stubAddr string

func (s stubAddr) Network() string { return "tcp" }
func (s stubAddr) String() string  { return string(s) }

func TestPeerAllowlistCIDR(t *testing.T) {
	a := NewPeerAllowlist("10.0.0.0/8,127.0.0.1")
	if !a.Allows(stubAddr("10.1.2.3:1514")) {
		t.Fatal("10.1.2.3 should be allowed")
	}
	if a.Allows(stubAddr("192.168.1.1:1514")) {
		t.Fatal("192.168.1.1 should be denied")
	}
	if !a.Allows(stubAddr("127.0.0.1:9")) {
		t.Fatal("127.0.0.1 should be allowed")
	}
}

func TestPeerAllowlistEmptyAllowsAll(t *testing.T) {
	a := NewPeerAllowlist("")
	if a.Empty() != true {
		t.Fatal("expected empty")
	}
	if !a.Allows(stubAddr("203.0.113.1:1")) {
		t.Fatal("empty allowlist must allow")
	}
}

func TestPeerAllowlistHostname(t *testing.T) {
	// Resolve localhost — typically 127.0.0.1
	a := NewPeerAllowlist("localhost")
	ips, err := net.LookupIP("localhost")
	if err != nil || len(ips) == 0 {
		t.Skip("localhost lookup unavailable")
	}
	ip4 := ""
	for _, ip := range ips {
		if v := ip.To4(); v != nil {
			ip4 = v.String()
			break
		}
	}
	if ip4 == "" {
		t.Skip("no ipv4 for localhost")
	}
	if !a.Allows(stubAddr(ip4 + ":1514")) {
		t.Fatalf("%s should be allowed via localhost DNS", ip4)
	}
}
