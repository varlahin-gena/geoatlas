package config

import (
	"strings"
	"testing"
)

func TestValidateRequiresSecrets(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")
	cfg := Config{}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "API_OPS_TOKEN") {
		t.Fatalf("want token error, got %v", err)
	}
	cfg.APIOpsToken = "ops-token-sixteen"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "CLICKHOUSE_PASSWORD") {
		t.Fatalf("want CLICKHOUSE_PASSWORD error, got %v", err)
	}
	cfg.ClickHousePass = "clickhouse-pass1"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateAllowsAdminFallback(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")
	cfg := Config{
		APIAuthToken:   "unique-token-xyz0",
		ClickHousePass: "clickhouse-pass1",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if !cfg.UsingAdminFallback() {
		t.Fatal("expected admin fallback")
	}
	if cfg.BearerToken() != "unique-token-xyz0" {
		t.Fatalf("token=%q", cfg.BearerToken())
	}
}

func TestValidatePrefersOps(t *testing.T) {
	cfg := Config{APIOpsToken: "ops-token-sixteen", APIAuthToken: "unique-token-xyz0"}
	if cfg.BearerToken() != "ops-token-sixteen" || cfg.UsingAdminFallback() {
		t.Fatalf("ops=%q fallback=%v", cfg.BearerToken(), cfg.UsingAdminFallback())
	}
}

func TestValidateAllowsInsecureOverride(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "1")
	if err := (Config{}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsShortToken(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")
	cfg := Config{APIOpsToken: "short", ClickHousePass: "clickhouse-pass1"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "API_OPS_TOKEN") {
		t.Fatalf("got %v", err)
	}
}
