package model

import "testing"

func TestIsAllowedBlockedAndMetricKey(t *testing.T) {
	if !IsAllowedAction(" ALLOW ") || IsAllowedAction("deny") {
		t.Fatal("allowed")
	}
	if !IsBlockedAction("Drop") || IsBlockedAction("allow") {
		t.Fatal("blocked")
	}
	k := MetricKey{Type: "cpu", Target: "h", Name: "u"}
	if k.String() != "cpu.h.u" {
		t.Fatal(k.String())
	}
	_, _, _ = ContinentCenter("Europe")
	_, _, _ = ContinentCenter("ZZ")
	_, _, _ = CountryCenter("Russia")
	_, _, _ = CountryCenter("")
}
