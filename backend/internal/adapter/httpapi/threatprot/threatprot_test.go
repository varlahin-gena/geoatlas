package threatprot

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSuspiciousRequestPathTraversal(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/../../../etc/passwd", nil)
	if !SuspiciousRequest(req) {
		t.Fatal("expected path traversal detection")
	}
}

func TestSuspiciousRequestHeaderCRLF(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/events", nil)
	req.Header.Set("X-Test", "a\r\nInjected: true")
	if !SuspiciousRequest(req) {
		t.Fatal("expected header CRLF detection")
	}
}

func TestSuspiciousRequestClean(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/events?country=RU&q=proto:tcp", nil)
	if SuspiciousRequest(req) {
		t.Fatal("expected clean request")
	}
}

func TestValidateJSONStructureDepth(t *testing.T) {
	deep := `{"a":{"b":{"c":{"d":{"e":{"f":1}}}}}}`
	if err := ValidateJSONStructure([]byte(deep), 5, 500); err != ErrJSONTooDeep {
		t.Fatalf("err = %v, want ErrJSONTooDeep", err)
	}
}

func TestValidateJSONStructureStringLength(t *testing.T) {
	long := `{"x":"` + strings.Repeat("a", 501) + `"}`
	if err := ValidateJSONStructure([]byte(long), 5, 500); err != ErrJSONStringTooLong {
		t.Fatalf("err = %v, want ErrJSONStringTooLong", err)
	}
}

func TestRateLimiterBlocksBurst(t *testing.T) {
	lim := NewRateLimiter(1, 1)
	if !lim.Allow("k") {
		t.Fatal("first request should pass")
	}
	if lim.Allow("k") {
		t.Fatal("second immediate request should be limited")
	}
}
