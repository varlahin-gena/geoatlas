package config

import (
	"strings"
	"testing"
)

func TestParseReputationFeedsOverride(t *testing.T) {
	raw := `[{"name":"x","url":"https://example.com/x.netset","category":"c","format":"netset"}]`
	feeds, err := parseReputationFeedsJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 1 || feeds[0].Name != "x" {
		t.Fatalf("%+v", feeds)
	}
}

func TestParseReputationFeedsEmptyArray(t *testing.T) {
	feeds, err := parseReputationFeedsJSON("[]")
	if err != nil || feeds != nil {
		t.Fatalf("[]: feeds=%+v err=%v", feeds, err)
	}
}

func TestParseReputationFeedsInvalidJSON(t *testing.T) {
	_, err := parseReputationFeedsJSON("not-json")
	if err == nil {
		t.Fatal("expected JSON error")
	}
}

func TestFromEnvRejectsInvalidReputationFeeds(t *testing.T) {
	t.Setenv("REPUTATION_FEEDS", "not-json")
	cfg := FromEnv()
	err := cfg.ValidateConfig()
	if err == nil || !strings.Contains(err.Error(), "REPUTATION_FEEDS") {
		t.Fatalf("want REPUTATION_FEEDS in ValidateConfig error, got %v", err)
	}
}

func TestFromEnvAcceptsValidReputationFeeds(t *testing.T) {
	t.Setenv("REPUTATION_FEEDS", `[{"name":"x","url":"https://example.com/x.netset"}]`)
	cfg := FromEnv()
	if err := cfg.ValidateConfig(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.ReputationFeeds) != 1 || cfg.ReputationFeeds[0].Name != "x" {
		t.Fatalf("%+v", cfg.ReputationFeeds)
	}
}
