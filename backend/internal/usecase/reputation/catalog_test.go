package reputation

import (
	"strings"
	"testing"
)

func TestDefaultFeeds(t *testing.T) {
	feeds := DefaultFeeds()
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
		"bruteforceblocker",
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
	if _, ok := names["cruzit_web_attacks"]; ok {
		t.Fatal("cruzit_web_attacks FireHOL ipset removed upstream (404); should not be default")
	}
	if names["spamhaus_drop"] != "drop" || names["feodo"] != "c2" {
		t.Fatalf("categories: %+v", names)
	}
}

func TestCatalogFeeds(t *testing.T) {
	feeds := CatalogFeeds()
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
		if f.Name == "et_block_official" || strings.Contains(f.URL, "emergingthreats.net") {
			t.Fatalf("unstable et_block_official should not be in catalog: %+v", f)
		}
		seen[f.Name] = f.Format
	}
	if seen["spamhaus_drop_official"] != "spamhaus_json" {
		t.Fatalf("spamhaus official format: %q", seen["spamhaus_drop_official"])
	}
	if _, ok := seen["feodo_abusech"]; !ok {
		t.Fatal("missing feodo_abusech")
	}
	for _, need := range []string{
		"blocklist_de", "spamhaus_drop", "dshield", "feodo",
		"et_block", "ciarmy", "greensnow", "et_compromised", "bruteforceblocker",
	} {
		if _, ok := seen[need]; !ok {
			t.Fatalf("missing default seed in catalog: %s", need)
		}
	}
}

func TestWithoutRetired(t *testing.T) {
	in := []Feed{
		{Name: "spamhaus_drop", URL: "https://example.com/a", Category: "drop", Format: "netset"},
		{Name: "cruzit_web_attacks", URL: "https://example.com/b", Category: "attacks", Format: "netset"},
		{Name: "sslbl", URL: "https://example.com/c", Category: "c2", Format: "netset"},
		{Name: "et_block_official", URL: "https://example.com/d", Category: "block", Format: "netset"},
	}
	out, changed := WithoutRetired(in)
	if !changed {
		t.Fatal("expected changed")
	}
	if len(out) != 1 || out[0].Name != "spamhaus_drop" {
		t.Fatalf("%+v", out)
	}
}
