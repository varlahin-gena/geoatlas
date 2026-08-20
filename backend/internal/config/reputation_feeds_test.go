package config

import "testing"

func TestParseReputationFeedsOverride(t *testing.T) {
	raw := `[{"name":"x","url":"https://example.com/x.netset","category":"c","format":"netset"}]`
	feeds := parseReputationFeeds(raw)
	if len(feeds) != 1 || feeds[0].Name != "x" {
		t.Fatalf("%+v", feeds)
	}
}

func TestParseReputationFeedsInvalidReturnsNil(t *testing.T) {
	if feeds := parseReputationFeeds("not-json"); feeds != nil {
		t.Fatalf("%+v", feeds)
	}
	if feeds := parseReputationFeeds(""); feeds != nil {
		t.Fatalf("%+v", feeds)
	}
	if feeds := parseReputationFeeds("[]"); feeds != nil {
		t.Fatalf("%+v", feeds)
	}
}
