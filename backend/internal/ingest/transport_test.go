package ingest

import (
	"testing"
)

func TestResolveTransport(t *testing.T) {
	tests := []struct {
		line     string
		fallback string
		wantT    string
		wantP    string
	}{
		{"@@nm/udp/@@hello", "", "udp", "hello"},
		{"@@nm/tcp/@@hello", "", "tcp", "hello"},
		{"plain message", "udp", "udp", "plain message"},
		{"plain message", "", "", "plain message"},
	}
	for _, tc := range tests {
		gotT, gotP := ResolveTransport(tc.line, tc.fallback)
		if gotT != tc.wantT || gotP != tc.wantP {
			t.Fatalf("ResolveTransport(%q, %q) = (%q, %q), want (%q, %q)",
				tc.line, tc.fallback, gotT, gotP, tc.wantT, tc.wantP)
		}
	}
}
