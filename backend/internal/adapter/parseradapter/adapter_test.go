package parseradapter

import (
	"testing"

	"geoatlas/internal/parser"
)

func TestAdapterNilAndParse(t *testing.T) {
	var a *Adapter
	if r := a.ParseVerbose("x"); r.Reason != "parser unavailable" {
		t.Fatalf("%+v", r)
	}
	empty := New(nil)
	if r := empty.ParseVerbose("x"); r.Reason != "parser unavailable" {
		t.Fatalf("%+v", r)
	}
	reg := parser.NewRegistry(&parser.GenericKV{})
	ad := New(reg)
	_ = ad.ParseVerbose("src=1.2.3.4 dst=5.6.7.8")
	if !ad.ContainsIPv4("hello 1.2.3.4 world") {
		t.Fatal("ContainsIPv4")
	}
	if ad.ContainsIPv4("no ip here") {
		t.Fatal("false positive")
	}

	var pt *ParseTestAdapter
	if r := pt.ParseVerbose("x"); r.Reason != "parser unavailable" {
		t.Fatalf("%+v", r)
	}
	pta := NewParseTest(nil)
	if r := pta.ParseVerbose("x"); r.Reason != "parser unavailable" {
		t.Fatalf("%+v", r)
	}
	pta = NewParseTest(reg)
	_ = pta.ParseVerbose("src=1.1.1.1 dst=8.8.8.8")
	if len(pta.SamplesByVendor()) == 0 {
		t.Fatal("samples empty")
	}
}
