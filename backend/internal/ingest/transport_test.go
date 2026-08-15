package ingest

import (
	"testing"
)

func TestResolveTransportLegacy(t *testing.T) {
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

func TestResolveTransportAuth(t *testing.T) {
	const secret = "s3cret"
	tr, payload, ok := ResolveTransportAuth("@@nm/udp/"+secret+"/@@hello", "", secret)
	if !ok || tr != "udp" || payload != "hello" {
		t.Fatalf("got tr=%q payload=%q ok=%v", tr, payload, ok)
	}
	_, _, ok = ResolveTransportAuth("@@nm/udp/wrong/@@hello", "", secret)
	if ok {
		t.Fatal("bad token must fail")
	}
	_, _, ok = ResolveTransportAuth("@@nm/udp/@@hello", "", secret)
	if ok {
		t.Fatal("legacy without token must fail when secret set")
	}
	tr, payload, ok = ResolveTransportAuth("@@nm/tcp/@@hello", "tcp", "")
	if !ok || tr != "tcp" || payload != "hello" {
		t.Fatalf("legacy ok when secret empty: tr=%q payload=%q ok=%v", tr, payload, ok)
	}
	_, _, ok = ResolveTransportAuth("plain", "", secret)
	if ok {
		t.Fatal("plain line must fail when secret required")
	}
	tr, payload, ok = ResolveTransportAuth("plain http body", "http", secret)
	if !ok || tr != "http" || payload != "plain http body" {
		t.Fatalf("http fallback must skip marker secret: tr=%q payload=%q ok=%v", tr, payload, ok)
	}
}
