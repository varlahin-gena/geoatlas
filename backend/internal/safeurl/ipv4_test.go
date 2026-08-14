package safeurl

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func TestValidateHTTPURLPublicIPv4Literal(t *testing.T) {
	if err := ValidateHTTPURL("https://8.8.8.8/path"); err != nil {
		t.Fatalf("public literal: %v", err)
	}
}

func TestValidateHTTPURLRejectsPrivate(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1/",
		"http://10.0.0.1/x",
		"http://192.168.1.1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://100.64.0.1/",
	} {
		if err := ValidateHTTPURL(raw); err == nil {
			t.Fatalf("expected reject for %s", raw)
		}
	}
}

func TestValidateHTTPURLRejectsIPv6(t *testing.T) {
	if err := ValidateHTTPURL("http://[2001:db8::1]/"); err == nil {
		t.Fatal("expected ipv6 reject")
	}
}

func TestValidateHTTPURLRejectsBlockedHostname(t *testing.T) {
	if err := ValidateHTTPURL("http://metadata.google.internal/"); err == nil {
		t.Fatal("expected metadata reject")
	}
}

func TestValidateHTTPURLLookup(t *testing.T) {
	orig := LookupIPv4
	t.Cleanup(func() { LookupIPv4 = orig })

	LookupIPv4 = func(host string) ([]net.IP, error) {
		if host == "evil.example" {
			return []net.IP{net.ParseIP("10.1.2.3")}, nil
		}
		if host == "ok.example" {
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		}
		if host == "aaaa-only.example" {
			return nil, nil
		}
		return nil, fmtError("nxdomain")
	}

	if err := ValidateHTTPURL("https://evil.example/list"); err == nil {
		t.Fatal("expected private resolve reject")
	}
	if err := ValidateHTTPURL("https://ok.example/list"); err != nil {
		t.Fatalf("public host: %v", err)
	}
	if err := ValidateHTTPURL("https://aaaa-only.example/"); err == nil {
		t.Fatal("expected no-ipv4 reject")
	}
}

type fmtError string

func (e fmtError) Error() string { return string(e) }

func TestSecureHTTPClientRejectsPrivateRedirect(t *testing.T) {
	client := SecureHTTPClient(&http.Client{Timeout: time.Second})
	req, err := http.NewRequest(http.MethodGet, "https://example.com/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	via := []*http.Request{req}
	redir, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(redir, via); err == nil {
		t.Fatal("expected redirect to private IP to fail")
	}
}

func TestSecureHTTPClientAllowsPublicRedirect(t *testing.T) {
	client := SecureHTTPClient(&http.Client{Timeout: time.Second})
	req, err := http.NewRequest(http.MethodGet, "https://example.com/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	via := []*http.Request{req}
	redir, err := http.NewRequest(http.MethodGet, "https://8.8.8.8/next", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(redir, via); err != nil {
		t.Fatalf("public redirect: %v", err)
	}
}
