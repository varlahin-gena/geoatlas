package parser

import (
	"fmt"
	"sort"
)

type prioritized struct {
	priority int
	parser   Parser
}

// BuildRegistry собирает registry: enabled DB-rules + built-in Cisco, по priority ASC.
// При равном priority порядок стабилен: сначала rules в переданном порядке, затем builtins.
func BuildRegistry(rules []RuleRecord) (*Registry, error) {
	items := make([]prioritized, 0, len(rules)+2)

	for _, rec := range rules {
		if !rec.Enabled {
			continue
		}
		rp, err := NewRuleParser(rec)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", rec.ID, err)
		}
		items = append(items, prioritized{priority: rec.Priority, parser: rp})
	}

	items = append(items,
		prioritized{priority: PriorityCiscoFTD, parser: &CiscoFTD{}},
		prioritized{priority: PriorityCiscoASA, parser: &CiscoASA{}},
	)

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].priority < items[j].priority
	})

	parsers := make([]Parser, len(items))
	for i := range items {
		parsers[i] = items[i].parser
	}
	return NewRegistry(parsers...), nil
}

// SamplesFromRules объединяет samples из rules + Cisco corpus.
func SamplesFromRules(rules []RuleRecord) map[string][]string {
	out := make(map[string][]string)
	for _, rec := range rules {
		if !rec.Enabled {
			continue
		}
		for _, line := range rec.Spec.Samples {
			if line == "" {
				continue
			}
			out[rec.Vendor] = append(out[rec.Vendor], line)
		}
	}
	for _, s := range sampleCorpus {
		if s.Vendor == "cisco-asa" || s.Vendor == "cisco-ftd" {
			out[s.Vendor] = append(out[s.Vendor], s.Line)
		}
	}
	return out
}
