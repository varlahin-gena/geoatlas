package parser

import (
	"maps"
	"regexp"
	"strings"
	"testing"
)

// Прежний regexp-парсер: эталон поведения walkCEFExt.
var cefKeyValueRE = regexp.MustCompile(`(?:^|\s)([A-Za-z0-9_.-]+)=`)

func parseCEFExtensionLegacy(ext string) map[string]string {
	matches := cefKeyValueRE.FindAllStringSubmatchIndex(ext, -1)
	out := make(map[string]string, len(matches))
	for i, m := range matches {
		key := ext[m[2]:m[3]]
		valStart := m[1]
		valEnd := len(ext)
		if i+1 < len(matches) {
			valEnd = matches[i+1][0]
		}
		out[key] = strings.TrimSpace(ext[valStart:valEnd])
	}
	return out
}

func TestWalkCEFExtMatchesLegacy(t *testing.T) {
	t.Parallel()
	for _, s := range sampleCorpus {
		if !strings.Contains(s.Line, "CEF:") {
			continue
		}
		_, ext, _, ok := parseCEF(s.Line)
		if !ok {
			t.Fatalf("%s/%s: parseCEF failed", s.Vendor, s.Desc)
		}
		got := map[string]string{}
		walkCEFExt(ext, func(k, v string) { got[k] = v })
		want := parseCEFExtensionLegacy(ext)
		if !maps.Equal(got, want) {
			t.Errorf("%s/%s: walkCEFExt != legacy\ngot  %v\nwant %v", s.Vendor, s.Desc, got, want)
		}
	}
}

func TestParseCEFHeader(t *testing.T) {
	t.Parallel()
	h, ext, prefix, ok := parseCEF(`Dec 27 11:07:55 FGT-A-LOG CEF: 0|Fortinet|Fortigate|v6.0.3|00013|traffic:forward close|3|src=10.1.100.11 dst=52.53.140.235`)
	if !ok {
		t.Fatal("parseCEF = false")
	}
	if prefix != "Dec 27 11:07:55 FGT-A-LOG " {
		t.Errorf("prefix = %q", prefix)
	}
	if h.Version != "0" || h.Vendor != "Fortinet" || h.Product != "Fortigate" {
		t.Errorf("header = %+v", h)
	}
	if h.Name != "traffic:forward close" || h.Severity != "3" {
		t.Errorf("header name/sev = %+v", h)
	}
	if !strings.HasPrefix(ext, "src=") {
		t.Errorf("ext = %q", ext)
	}
}

func TestCEFVendor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		line string
		want string
		ok   bool
	}{
		{`CEF:0|Usergate|UTM|6|traffic|firewall|1|src=1.1.1.1`, "Usergate", true},
		{`CEF: 0|Fortinet|Fortigate|v6|1|n|3|src=1.1.1.1`, "Fortinet", true},
		{`CEF:0|FORTINET|x|1|2|3|4|src=1`, "FORTINET", true},
		{`not cef`, "", false},
		{`CEF:0|Fortinet`, "", false},
	}
	for _, tt := range tests {
		got, ok := cefVendor(tt.line)
		if ok != tt.ok || got != tt.want {
			t.Errorf("cefVendor(%q) = %q, %v; want %q, %v", tt.line, got, ok, tt.want, tt.ok)
		}
	}
}

func TestCEFCanParseVendorCase(t *testing.T) {
	t.Parallel()
	fg := &FortigateCEF{}
	ug := &UserGateCEF{}
	if !fg.CanParse(`CEF:0|FORTINET|Fortigate|v|1|n|3|src=1.1.1.1 dst=2.2.2.2`) {
		t.Error("Fortigate.CanParse FORTINET = false")
	}
	if !ug.CanParse(`CEF:0|USERGATE|UTM|6|traffic|firewall|1|src=1.1.1.1 dst=2.2.2.2`) {
		t.Error("UserGate.CanParse USERGATE = false")
	}
	if fg.CanParse(`CEF:0|Usergate|UTM|6|traffic|firewall|1|src=1.1.1.1 dst=2.2.2.2`) {
		t.Error("Fortigate.CanParse Usergate = true")
	}
}

func TestWalkCEFExtSpacedValue(t *testing.T) {
	t.Parallel()
	ext := `cs1Label=Rule cs1=Allow trusted to untrusted src=10.10.10.10`
	got := map[string]string{}
	walkCEFExt(ext, func(k, v string) { got[k] = v })
	if got["cs1"] != "Allow trusted to untrusted" {
		t.Errorf("cs1 = %q", got["cs1"])
	}
	if got["src"] != "10.10.10.10" {
		t.Errorf("src = %q", got["src"])
	}
	if got["cs1Label"] != "Rule" {
		t.Errorf("cs1Label = %q", got["cs1Label"])
	}
}
