package parser

import (
	"testing"
	"time"
)

func TestRuleParserKVAndHelpers(t *testing.T) {
	rec := DefaultSeedRules()[2] // generic
	rp, err := NewRuleParser(rec)
	if err != nil {
		t.Fatal(err)
	}
	if !rp.CanParse("src=1.2.3.4 dst=5.6.7.8") {
		t.Fatal("CanParse")
	}
	tl, ok := rp.Parse("src=10.0.0.1 dst=10.0.0.2 spt=1 dpt=2 act=Deny proto=udp rule=R out=9 in=8 cn1=1 cn2=2")
	if !ok {
		t.Fatal("Parse")
	}
	if tl.Action != "deny" || tl.Proto != "UDP" || tl.BytesSent != 9 {
		t.Fatalf("%+v", tl)
	}
	if rp.Record().ID != SeedIDGeneric {
		t.Fatal(rp.Record().ID)
	}

	fg := DefaultSeedRules()[1]
	fp, err := NewRuleParser(fg)
	if err != nil {
		t.Fatal(err)
	}
	line := Samples()[1].Line // fortigate sample
	if !fp.CanParse(line) {
		t.Fatal("fg CanParse")
	}
	if _, ok := fp.Parse(line); !ok {
		t.Fatal("fg Parse")
	}

	ts := parseRuleTime("1652344423", "s")
	if ts.Year() < 2020 {
		t.Fatal(ts)
	}
	ts = parseRuleTime("1652344423822", "ms")
	if time.Since(ts) < 0 && ts.Year() < 2020 {
		t.Fatal(ts)
	}
	_ = parseRuleTime("not-a-number", "auto")
	_ = parseRuleTime("", "")
	_ = joinRule("", "1")
	_ = joinRule("app", "0")
	_ = joinRule("app", "2")
	_ = pickZone("undefined", []string{"port1"}, true)

	if _, err := BuildRegistry([]RuleRecord{{
		ID: "bad", Name: "bad", Vendor: "bad", Enabled: true,
		Spec: RuleSpec{Kind: "nope", Fields: FieldMap{Src: "a", Dst: "b"}},
	}}); err == nil {
		t.Fatal("want build error")
	}

	rr := NewReloadable(nil)
	rr.Swap(nil)
	_ = rr.ParseVerbose("")
	_, _ = rr.Parse("src=1.1.1.1 dst=8.8.8.8")

	sm := SamplesFromRules(DefaultSeedRules())
	if len(sm["cisco-asa"]) == 0 || len(sm["usergate"]) == 0 {
		t.Fatalf("%v", sm)
	}
}
