package parser

import (
	"testing"
)

func TestRuleParserMatchesGoParsers(t *testing.T) {
	seeds := DefaultSeedRules()
	ruleReg, err := BuildRegistry(seeds)
	if err != nil {
		t.Fatal(err)
	}
	legacy := NewRegistry(
		&UserGateCEF{},
		&FortigateCEF{},
		&CiscoFTD{},
		&CiscoASA{},
		&GenericKV{},
	)

	for _, s := range Samples() {
		t.Run(s.Vendor+"/"+s.Desc, func(t *testing.T) {
			got := ruleReg.ParseVerbose(s.Line)
			want := legacy.ParseVerbose(s.Line)
			if got.OK != want.OK || got.Skipped != want.Skipped {
				t.Fatalf("OK/Skipped: got (%v,%v) want (%v,%v) reason=%q/%q",
					got.OK, got.Skipped, want.OK, want.Skipped, got.Reason, want.Reason)
			}
			if got.Vendor != want.Vendor {
				t.Errorf("vendor: got %q want %q", got.Vendor, want.Vendor)
			}
			if !got.OK {
				return
			}
			checkTL(t, "rule-vs-go", got.Log, tlWant{
				vendor: want.Log.Vendor, src: want.Log.SrcIP, dst: want.Log.DstIP,
				sport: want.Log.SrcPort, dport: want.Log.DstPort,
				action: want.Log.Action, proto: want.Log.Proto, rule: want.Log.Rule,
				szone: want.Log.SrcZone, dzone: want.Log.DstZone,
				scountry: want.Log.SrcCountry, dcty: want.Log.DstCountry,
				device:    want.Log.Device,
				bytesSent: want.Log.BytesSent, bytesRecv: want.Log.BytesRecv,
				pktSent: want.Log.PacketsSent, pktRecv: want.Log.PacketsRecv,
			})
		})
	}
}

func TestRuleSpecValidate(t *testing.T) {
	ok := DefaultSeedRules()[0].Spec
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := RuleSpec{Kind: RuleKindCEF, Match: RuleMatch{}, Fields: FieldMap{Src: "src", Dst: "dst"}}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected error for empty cef_vendor")
	}
}
