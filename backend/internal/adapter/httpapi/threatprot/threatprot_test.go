package threatprot

import (
	"fmt"
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

func TestSuspiciousRequestEncodedTraversal(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/%2e%2e/%2e%2e/etc/passwd", nil)
	if !SuspiciousRequest(req) {
		t.Fatal("expected encoded path traversal detection")
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

func TestValidateJSONStructureObjectNameLength(t *testing.T) {
	name := strings.Repeat("k", MaxJSONObjectEntryNameLength+1)
	payload := `{"` + name + `":1}`
	if err := ValidateJSONStructure([]byte(payload), 5, 500); err != ErrJSONObjectNameTooLong {
		t.Fatalf("err = %v, want ErrJSONObjectNameTooLong", err)
	}
}

func TestValidateJSONStructureObjectEntryCount(t *testing.T) {
	var b strings.Builder
	b.WriteByte('{')
	for i := 0; i < MaxJSONObjectEntryCount+1; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `"k%d":1`, i)
	}
	b.WriteByte('}')
	if err := ValidateJSONStructure([]byte(b.String()), 5, 500); err != ErrJSONObjectEntriesExceed {
		t.Fatalf("err = %v, want ErrJSONObjectEntriesExceed", err)
	}
}

func TestValidateJSONStructureArrayElementCount(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"ids":[`)
	for i := 0; i < MaxJSONArrayElementCount+1; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('1')
	}
	b.WriteString(`]}`)
	if err := ValidateJSONStructure([]byte(b.String()), 5, 500); err != ErrJSONArrayElementsExceed {
		t.Fatalf("err = %v, want ErrJSONArrayElementsExceed", err)
	}
}

func TestValidateJSONStructureAcceptsNormal(t *testing.T) {
	payload := `{"username":"admin","password":"secret","roles":["a","b"]}`
	if err := ValidateJSONStructure([]byte(payload), 5, 500); err != nil {
		t.Fatalf("unexpected err: %v", err)
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
