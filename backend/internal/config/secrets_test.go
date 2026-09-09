package config

import (
	"strings"
	"testing"
)

func okSecurityCfg() Config {
	return Config{
		Auth: AuthConfig{
			APIAuthToken:  "unique-token-xyz0", // 16+
			SessionSecret: "ok-secret-not-placeholder",
		},
		Ingest:     IngestConfig{SharedSecret: "ingest-secret-ok"},
		ClickHouse: ClickHouseConfig{Password: "clickhouse-pass1"},
	}
}

func TestValidateSecurityRejectsPlaceholders(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")

	cfg := okSecurityCfg()
	cfg.Auth.APIAuthToken = "dev-insecure-change-me"
	if err := cfg.ValidateSecurity(); err == nil {
		t.Fatal("expected error for insecure API token")
	}

	cfg = okSecurityCfg()
	cfg.Auth.SessionSecret = "dev-session-secret-change-me"
	if err := cfg.ValidateSecurity(); err == nil {
		t.Fatal("expected error for insecure session secret")
	}
}

func TestValidateSecurityRequiresIngestSecret(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")
	cfg := okSecurityCfg()
	cfg.Ingest.SharedSecret = ""
	if err := cfg.ValidateSecurity(); err == nil {
		t.Fatal("expected error for missing INGEST_SHARED_SECRET")
	}
	t.Setenv("GA_ALLOW_INSECURE", "1")
	if err := cfg.ValidateSecurity(); err != nil {
		t.Fatalf("GA_ALLOW_INSECURE should permit empty ingest secret: %v", err)
	}
}

func TestValidateSecurityRequiresClickHousePassword(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")
	cfg := okSecurityCfg()
	cfg.ClickHouse.Password = ""
	if err := cfg.ValidateSecurity(); err == nil {
		t.Fatal("expected error for missing CLICKHOUSE_PASSWORD")
	}
	t.Setenv("GA_ALLOW_INSECURE", "1")
	if err := cfg.ValidateSecurity(); err != nil {
		t.Fatalf("GA_ALLOW_INSECURE should permit empty CH password: %v", err)
	}
}

func TestValidateSecurityRejectsShortSecrets(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")
	cfg := okSecurityCfg()
	cfg.Auth.SessionSecret = "short"
	if err := cfg.ValidateSecurity(); err == nil || !strings.Contains(err.Error(), "SESSION_SECRET") {
		t.Fatalf("want SESSION_SECRET min length error, got %v", err)
	}
	cfg = okSecurityCfg()
	cfg.Auth.APIAuthToken = "tooshort"
	if err := cfg.ValidateSecurity(); err == nil || !strings.Contains(err.Error(), "API_AUTH_TOKEN") {
		t.Fatalf("want API_AUTH_TOKEN min length error, got %v", err)
	}
	t.Setenv("GA_ALLOW_INSECURE", "1")
	cfg = okSecurityCfg()
	cfg.Auth.SessionSecret = "x"
	if err := cfg.ValidateSecurity(); err != nil {
		t.Fatalf("GA_ALLOW_INSECURE should permit short secrets: %v", err)
	}
}

func TestSecurityWarningsWeakSeedPasswords(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")
	cfg := Config{
		Auth: AuthConfig{
			AdminUser:        "admin",
			AdminPassword:    "admin",
			OperatorUser:     "operator",
			OperatorPassword: "strong-op!",
		},
	}
	w := cfg.SecurityWarnings()
	if len(w) != 1 {
		t.Fatalf("want 1 warning, got %#v", w)
	}
}

func TestValidateSecurityAllowsOverride(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "1")
	cfg := Config{
		Auth: AuthConfig{
			APIAuthToken:  "dev-insecure-change-me",
			SessionSecret: "dev-session-secret-change-me",
		},
	}
	if err := cfg.ValidateSecurity(); err != nil {
		t.Fatalf("GA_ALLOW_INSECURE should permit placeholders: %v", err)
	}
}

func TestValidateSecurityDisabledAuthRequiresInsecure(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")
	cfg := Config{
		Auth: AuthConfig{
			APIAuthDisabled: true,
			Disabled:        true,
		},
		Ingest:     IngestConfig{SharedSecret: "ingest-secret-ok"},
		ClickHouse: ClickHouseConfig{Password: "clickhouse-pass1"},
	}
	if err := cfg.ValidateSecurity(); err == nil {
		t.Fatal("expected error when *_DISABLED without GA_ALLOW_INSECURE")
	}

	t.Setenv("GA_ALLOW_INSECURE", "1")
	if err := cfg.ValidateSecurity(); err != nil {
		t.Fatalf("GA_ALLOW_INSECURE should permit *_DISABLED: %v", err)
	}
}

func TestValidateSecurityAPIAuthDisabledAlone(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")
	cfg := Config{
		Auth: AuthConfig{
			APIAuthDisabled: true,
			SessionSecret:   "unique-session-secret",
		},
		Ingest:     IngestConfig{SharedSecret: "ingest-secret-ok"},
		ClickHouse: ClickHouseConfig{Password: "clickhouse-pass1"},
	}
	if err := cfg.ValidateSecurity(); err == nil {
		t.Fatal("expected error for API_AUTH_DISABLED without GA_ALLOW_INSECURE")
	}
}

func TestAPIAuthTokensIncludesPrevious(t *testing.T) {
	cfg := Config{Auth: AuthConfig{APIAuthToken: "current", APIAuthPreviousToken: "previous"}}
	got := cfg.APIAuthTokens()
	if len(got) != 2 || got[0] != "current" || got[1] != "previous" {
		t.Fatalf("tokens=%#v", got)
	}
	cfg.Auth.APIAuthPreviousToken = "current"
	got = cfg.APIAuthTokens()
	if len(got) != 1 || got[0] != "current" {
		t.Fatalf("dedupe want 1, got %#v", got)
	}
}

func TestValidateSecurityRejectsPreviousPlaceholder(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")
	cfg := okSecurityCfg()
	cfg.Auth.APIAuthPreviousToken = "dev-insecure-change-me"
	if err := cfg.ValidateSecurity(); err == nil {
		t.Fatal("expected error for insecure previous token")
	}
}

func TestValidateSecurityRejectsOpsEqualsAdmin(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")
	cfg := okSecurityCfg()
	cfg.Auth.APIOpsToken = cfg.Auth.APIAuthToken
	if err := cfg.ValidateSecurity(); err == nil || !strings.Contains(err.Error(), "API_OPS_TOKEN") {
		t.Fatalf("want distinct ops token error, got %v", err)
	}
}

func TestValidateSecurityOK(t *testing.T) {
	t.Setenv("GA_ALLOW_INSECURE", "")
	cfg := okSecurityCfg()
	cfg.Auth.APIOpsToken = "ops-token-sixteen"
	if err := cfg.ValidateSecurity(); err != nil {
		t.Fatal(err)
	}
}
