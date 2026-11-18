package parseradapter

import (
	"context"
	"testing"

	"geoatlas/internal/parser"
)

type memRules struct {
	rules []parser.RuleRecord
	err   error
}

func (m memRules) List(context.Context) ([]parser.RuleRecord, error) {
	return m.rules, m.err
}

func TestReloaderAndSamplesHolder(t *testing.T) {
	reg := parser.NewReloadable(nil)
	samples := NewSamplesHolder(nil)
	if len(samples.SamplesByVendor()) != 0 {
		t.Fatal("empty holder")
	}
	var nilHolder *SamplesHolder
	_ = nilHolder.SamplesByVendor()
	nilHolder.Store(nil)

	r := NewReloader(memRules{rules: parser.DefaultSeedRules()}, reg, samples)
	if err := r.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	res := reg.ParseVerbose(`src=192.0.2.1 dst=198.51.100.2 act=allow`)
	if !res.OK {
		t.Fatalf("%+v", res)
	}
	if len(samples.SamplesByVendor()["usergate"]) == 0 {
		t.Fatal("usergate samples")
	}

	bad := NewReloader(nil, nil, nil)
	if err := bad.Reload(context.Background()); err == nil {
		t.Fatal("expected error")
	}

	ad := New(reg)
	_ = ad.ParseVerbose("src=1.1.1.1 dst=8.8.8.8")
	pta := NewParseTest(reg, samples)
	if len(pta.SamplesByVendor()["cisco-asa"]) == 0 {
		t.Fatal("cisco samples")
	}
}
