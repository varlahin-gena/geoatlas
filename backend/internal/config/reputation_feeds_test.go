package config

import "testing"

func TestDefaultReputationFeeds(t *testing.T) {
	feeds := DefaultReputationFeeds()
	if len(feeds) != 1 {
		t.Fatalf("want 1 default feed, got %d", len(feeds))
	}
	if feeds[0].Name != "firehol_level1" {
		t.Fatalf("name: %s", feeds[0].Name)
	}
	if feeds[0].URL != DefaultFireholLevel1URL {
		t.Fatalf("url: %s", feeds[0].URL)
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
	if len(feeds) != 1 || feeds[0].Name != "firehol_level1" {
		t.Fatalf("%+v", feeds)
	}
}
