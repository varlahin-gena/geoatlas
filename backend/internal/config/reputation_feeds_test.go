package config

import "testing"

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
	for _, need := range []string{"spamhaus_drop", "dshield", "feodo"} {
		if _, ok := names[need]; !ok {
			t.Fatalf("missing %s in defaults", need)
		}
	}
	if names["spamhaus_drop"] != "drop" || names["feodo"] != "c2" {
		t.Fatalf("categories: %+v", names)
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
