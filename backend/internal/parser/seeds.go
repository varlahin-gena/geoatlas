package parser

// Built-in priority slots (Cisco остаётся в Go).
const (
	PriorityCiscoFTD = 300
	PriorityCiscoASA = 310
)

// SeedRuleIDs — стабильные id seed-правил.
const (
	SeedIDUserGate  = "usergate"
	SeedIDFortigate = "fortigate"
	SeedIDGeneric   = "generic"
)

// DefaultSeedRules — UserGate / Fortigate / Generic, эквивалент прежних Go-парсеров.
func DefaultSeedRules() []RuleRecord {
	return []RuleRecord{
		{
			ID: SeedIDUserGate, Name: "UserGate CEF", Vendor: "usergate",
			Enabled: true, Priority: 100, Seed: true,
			Spec: RuleSpec{
				Kind:  RuleKindCEF,
				Match: RuleMatch{CEFVendor: "Usergate"},
				Fields: FieldMap{
					Src: "src", Dst: "dst", Spt: "spt", Dpt: "dpt",
					Act: "act", Proto: "proto", Rule: "cs1",
					SrcZone: "cs2", DstZone: "cs4",
					SrcCountry: "cs3", DstCountry: "cs5",
					// UserGate: in/cn1 = sent, out/cn2 = recv (инверт ArcSight).
					BytesSent: "in", BytesRecv: "out",
					PacketsSent: "cn1", PacketsRecv: "cn2",
					Time: "rt",
				},
				Transforms: Transforms{TimeUnit: "auto"},
				Samples:    samplesForVendor("usergate"),
			},
		},
		{
			ID: SeedIDFortigate, Name: "FortiGate CEF", Vendor: "fortigate",
			Enabled: true, Priority: 200, Seed: true,
			Spec: RuleSpec{
				Kind:  RuleKindCEF,
				Match: RuleMatch{CEFVendor: "Fortinet"},
				Fields: FieldMap{
					Src: "src", Dst: "dst", Spt: "spt", Dpt: "dpt",
					Act: "act", Proto: "proto",
					SrcCountry: "FTNTFGTsrccountry", DstCountry: "FTNTFGTdstcountry",
					BytesSent: "out", BytesRecv: "in",
					PacketsSent: "FTNTFGTsentpkt", PacketsRecv: "FTNTFGTrcvdpkt",
					Time: "FTNTFGTeventtime",
				},
				Transforms: Transforms{
					ProtoMapNumber:     true,
					TimeUnit:           "auto",
					SkipUndefinedZones: true,
					SrcZoneFallback: []string{
						"FTNTFGTsrcintfrole", "deviceInboundInterface", "FTNTFGTsrcintf",
					},
					DstZoneFallback: []string{
						"FTNTFGTdstintfrole", "deviceOutboundInterface", "FTNTFGTdstintf",
					},
					RuleJoin: &RuleJoin{Primary: "app", Policy: "FTNTFGTpolicyid"},
				},
				Samples: samplesForVendor("fortigate"),
			},
		},
		{
			ID: SeedIDGeneric, Name: "Generic key=value", Vendor: "generic",
			Enabled: true, Priority: 900, Seed: true,
			Spec: RuleSpec{
				Kind:  RuleKindKV,
				Match: RuleMatch{RequiredKeys: []string{"src", "dst"}},
				Fields: FieldMap{
					Src: "src", Dst: "dst", Spt: "spt", Dpt: "dpt",
					Act: "act", Proto: "proto", Rule: "rule",
					BytesSent: "out", BytesRecv: "in",
					PacketsSent: "cn1", PacketsRecv: "cn2",
				},
				Samples: samplesForVendor("generic"),
			},
		},
	}
}

func samplesForVendor(vendor string) []string {
	var out []string
	for _, s := range sampleCorpus {
		if s.Vendor == vendor {
			out = append(out, s.Line)
		}
	}
	return out
}

// BuiltinParsers — Cisco ASA/FTD (не редактируются через GUI).
func BuiltinParsers() []Parser {
	return []Parser{
		&CiscoFTD{},
		&CiscoASA{},
	}
}

// BuiltinPriorities — фиксированные priority для List API.
func BuiltinPriorities() map[string]int {
	return map[string]int{
		"cisco-ftd": PriorityCiscoFTD,
		"cisco-asa": PriorityCiscoASA,
	}
}
