package parser

import (
	"maps"
	"strings"
	"testing"
)

// Прежний Split-парсер: эталон поведения walkFTDKV.
func parseFTDKVLegacy(s string) map[string]string {
	out := make(map[string]string, 32)
	for _, p := range strings.Split(s, ", ") {
		if kv := strings.SplitN(p, ":", 2); len(kv) == 2 {
			out[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return out
}

func TestWalkFTDKVMatchesLegacy(t *testing.T) {
	t.Parallel()
	for _, s := range sampleCorpus {
		if s.Vendor != "cisco-ftd" || !strings.Contains(s.Line, "SrcIP:") {
			continue
		}
		idx := -1
		for _, marker := range []string{
			"EventPriority:", "DeviceUUID:", "AccessControlRuleAction:", "SrcIP:",
		} {
			if i := strings.Index(s.Line, marker); i != -1 && (idx == -1 || i < idx) {
				idx = i
			}
		}
		if idx < 0 {
			t.Fatalf("%s: no KV start", s.Desc)
		}
		raw := s.Line[idx:]
		got := map[string]string{}
		walkFTDKV(raw, func(k, v string) { got[k] = v })
		want := parseFTDKVLegacy(raw)
		if !maps.Equal(got, want) {
			t.Errorf("%s: walkFTDKV != legacy\ngot  %v\nwant %v", s.Desc, got, want)
		}
	}
}

func TestWalkFTDKVSpacedKeyAndValue(t *testing.T) {
	t.Parallel()
	raw := `AccessControlRuleAction: Allow, Prefilter Policy: Prefilter, NAPPolicy: Balanced Security and Connectivity`
	got := map[string]string{}
	walkFTDKV(raw, func(k, v string) { got[k] = v })
	if got["AccessControlRuleAction"] != "Allow" {
		t.Errorf("action = %q", got["AccessControlRuleAction"])
	}
	if got["Prefilter Policy"] != "Prefilter" {
		t.Errorf("Prefilter Policy = %q", got["Prefilter Policy"])
	}
	if got["NAPPolicy"] != "Balanced Security and Connectivity" {
		t.Errorf("NAPPolicy = %q", got["NAPPolicy"])
	}
}
