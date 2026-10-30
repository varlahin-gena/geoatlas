package safeurl

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

// Capture package default before other tests swap the var.
var realLookupIPv4 = LookupIPv4

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

func TestResolvePublicIPv4RejectsPrivateAtDialTime(t *testing.T) {
	orig := LookupIPv4
	t.Cleanup(func() { LookupIPv4 = orig })

	LookupIPv4 = func(host string) ([]net.IP, error) {
		if host == "rebind.example" {
			return []net.IP{net.ParseIP("169.254.169.254")}, nil
		}
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}

	if _, err := resolvePublicIPv4("rebind.example"); err == nil {
		t.Fatal("expected metadata IP reject at dial resolve")
	}
	ips, err := resolvePublicIPv4("ok.example")
	if err != nil || len(ips) != 1 || ips[0].String() != "93.184.216.34" {
		t.Fatalf("got ips=%v err=%v", ips, err)
	}
}

func TestSecureHTTPClientPinsDialContext(t *testing.T) {
	client := SecureHTTPClient(&http.Client{Timeout: time.Second})
	tr, ok := client.Transport.(*http.Transport)
	if !ok || tr.DialContext == nil {
		t.Fatal("expected Transport with DialContext")
	}
}

func TestParseHTTPURLValidation(t *testing.T) {
	for _, raw := range []string{"", "  ", "ftp://x/", "http://", "http://user:pass@8.8.8.8/"} {
		if err := ValidateHTTPURL(raw); err == nil {
			t.Fatalf("expected reject for %q", raw)
		}
	}
	if err := ValidateHTTPURL("://bad"); err == nil {
		t.Fatal("invalid url")
	}
}

func TestValidateBlockedHostVariants(t *testing.T) {
	for _, host := range []string{"metadata", "kubernetes.default", "kubernetes.default.svc"} {
		if err := ValidateHTTPURL("http://" + host + "/"); err == nil {
			t.Fatalf("expected block %s", host)
		}
	}
}

func TestResolvePublicIPv4EmptyAndLookupErr(t *testing.T) {
	if _, err := resolvePublicIPv4(""); err == nil {
		t.Fatal("empty host")
	}
	orig := LookupIPv4
	t.Cleanup(func() { LookupIPv4 = orig })
	LookupIPv4 = func(string) ([]net.IP, error) { return nil, fmtError("nxdomain") }
	if _, err := resolvePublicIPv4("missing.example"); err == nil {
		t.Fatal("lookup err")
	}
	LookupIPv4 = func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("2001:db8::1")}, nil
	}
	if _, err := resolvePublicIPv4("v6-only.example"); err == nil {
		t.Fatal("no ipv4 after filter")
	}
}

func TestSecureHTTPClientNilBaseAndRedirects(t *testing.T) {
	client := SecureHTTPClient(nil)
	if client == nil || client.CheckRedirect == nil {
		t.Fatal("nil base")
	}
	req, _ := http.NewRequest(http.MethodGet, "https://8.8.8.8/", nil)
	via := make([]*http.Request, MaxRedirects)
	for i := range via {
		via[i] = req
	}
	if err := client.CheckRedirect(req, via); err == nil {
		t.Fatal("too many redirects")
	}
	bad := &http.Request{} // URL nil
	if err := client.CheckRedirect(bad, []*http.Request{req}); err == nil {
		t.Fatal("missing url")
	}
}

func TestCloneTransportNonHTTP(t *testing.T) {
	tr := cloneTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, nil
	}))
	if tr == nil {
		t.Fatal("clone fallback")
	}
	if cloneTransport(nil) == nil {
		t.Fatal("nil base transport")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDefaultDialContextRejects(t *testing.T) {
	if _, err := defaultDialContext(context.Background(), "tcp", "not-a-hostport"); err == nil {
		t.Fatal("split host port")
	}
	if _, err := defaultDialContext(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("private dial resolve")
	}
}

func TestRealLookupIPv4Default(t *testing.T) {
	orig := LookupIPv4
	t.Cleanup(func() { LookupIPv4 = orig })
	LookupIPv4 = realLookupIPv4

	ips, err := LookupIPv4("127.0.0.1")
	if err != nil {
		t.Fatalf("lookup literal: %v", err)
	}
	if len(ips) == 0 || ips[0].To4() == nil {
		t.Fatalf("ips=%v", ips)
	}
	if _, err := LookupIPv4("this-host-should-not-resolve.invalid"); err == nil {
		t.Fatal("want nxdomain")
	}
}

func TestCloneTransportHTTPTransport(t *testing.T) {
	base := http.DefaultTransport.(*http.Transport)
	tr := cloneTransport(base)
	if tr == nil || tr == base {
		t.Fatal("expected clone")
	}
}

func TestSecureHTTPClientUsesCustomTransport(t *testing.T) {
	baseTr := http.DefaultTransport.(*http.Transport).Clone()
	client := SecureHTTPClient(&http.Client{Timeout: time.Second, Transport: baseTr})
	tr, ok := client.Transport.(*http.Transport)
	if !ok || tr.DialContext == nil {
		t.Fatal("dial pinned")
	}
}

func TestValidateParsedEmptyHostname(t *testing.T) {
	u, err := parseHTTPURL("http://8.8.8.8/")
	if err != nil {
		t.Fatal(err)
	}
	u.Host = ""
	if err := validateParsed(u); err == nil {
		t.Fatal("empty hostname")
	}
}

func TestDefaultDialContextDialFail(t *testing.T) {
	origL := LookupIPv4
	t.Cleanup(func() { LookupIPv4 = origL })
	LookupIPv4 = func(string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.1")}, nil // TEST-NET-3
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := defaultDialContext(ctx, "tcp", "ok.example:9")
	if err == nil {
		t.Fatal("want canceled dial error")
	}
}
