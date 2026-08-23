package netutil

import (
	"net"
	"testing"
)

func TestIsNonPublicIPv4IP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
		{"127.0.0.1", true},
		{"169.254.1.1", true},
		{"100.64.0.1", true},
		{"100.127.255.255", true},
		{"0.0.0.0", true},
		{"8.8.8.8", false},
		{"1.2.3.4", false},
		{"100.63.255.255", false},
		{"100.128.0.1", false},
		{"::1", true},
	}
	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		if got := IsNonPublicIPv4IP(ip); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.ip, got, tc.want)
		}
	}
	if !IsNonPublicIPv4IP(nil) {
		t.Fatal("nil should be non-public")
	}
}
