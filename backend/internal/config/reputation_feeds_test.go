package config

import (
	"strings"
	"testing"
)

func TestDefaultReputationFeeds(t *testing.T) {
	feeds := DefaultReputationFeeds()
	if len(feeds) < 3 {
		t.Fatalf("want several default feeds, got %d", len(feeds))
	}
	names := map[string]string{}
	for _, f := range feeds {
		if f.Name == "" || f.URL == "" || f.Category == "" {
			t.Fatalf("incomplete feed: %+v", f)
		}
		if f.Name == "firehol_level1" || f.Name == "fullbogons" {
			t.Fatalf("aggregate/fullbogons should not be default: %s", f.Name)
		}
		names[f.Name] = f.Category
	}
	for _, need := range []string{
		"spamhaus_drop", "dshield", "feodo",
		"et_compromised",
		"bruteforceblocker", "cruzit_web_attacks",
	} {
		if _, ok := names[need]; !ok {
			t.Fatalf("missing %s in defaults", need)
		}
	}
	if _, ok := names["spamhaus_edrop"]; ok {
		t.Fatal("spamhaus_edrop should not be in defaults (merged into DROP)")
	}
	if _, ok := names["sslbl"]; ok {
		t.Fatal("sslbl FireHOL ipset removed upstream; should not be default")
	}
	if names["spamhaus_drop"] != "drop" || names["feodo"] != "c2" {
		t.Fatalf("categories: %+v", names)
	}
}

func TestCatalogReputationFeeds(t *testing.T) {
	feeds := CatalogReputationFeeds()
	if len(feeds) < 3 {
		t.Fatalf("want catalog presets, got %d", len(feeds))
	}
	seen := map[string]string{}
	for _, f := range feeds {
		if f.Name == "" || f.URL == "" || f.Format == "" {
			t.Fatalf("incomplete: %+v", f)
		}
		if f.Name == "tor_exits" || f.Name == "fullbogons" || strings.Contains(f.Name, "anonymous") {
			t.Fatalf("noisy list in catalog: %s", f.Name)
		}
		if f.Name == "sslbl_abusech" || strings.Contains(f.URL, "sslipblacklist") {
			t.Fatalf("deprecated SSLBL IP list in catalog: %+v", f)
		}
		seen[f.Name] = f.Format
	}
	if seen["spamhaus_drop_official"] != "spamhaus_json" {
		t.Fatalf("spamhaus official format: %q", seen["spamhaus_drop_official"])
	}
	if _, ok := seen["feodo_abusech"]; !ok {
		t.Fatal("missing feodo_abusech")
	}
}

func TestParseReputationFeedsOverride(t *testing.T) {
	raw := `[{"name":"x","url":"https://example.com/x.netset","category":"c","format":"netset"}]`
	feeds := parseReputationFeeds(raw)
	if len(feeds) != 1 || feeds[0].Name != "x" {
		t.Fatalf("%+v", feeds)
	}
}

func TestParseReputationFeedsInvalidFallsBack(t *testing.T) {
	feeds := parseReputationFeeds("not-json")
	if len(feeds) < 3 {
		t.Fatalf("%+v", feeds)
	}
}
