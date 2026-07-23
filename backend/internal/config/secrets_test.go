package config

import (
	"testing"
)

func TestValidateSecurityRejectsPlaceholders(t *testing.T) {
	t.Setenv("NM_ALLOW_INSECURE", "")

	cfg := Config{
		APIAuthToken:  "dev-insecure-change-me",
		SessionSecret: "ok-secret-not-placeholder",
	}
	if err := cfg.ValidateSecurity(); err == nil {
		t.Fatal("expected error for insecure API token")
	}

	cfg = Config{
		APIAuthToken:  "unique-token-xyz",
		SessionSecret: "dev-session-secret-change-me",
	}
	if err := cfg.ValidateSecurity(); err == nil {
		t.Fatal("expected error for insecure session secret")
	}
}

func TestSecurityWarningsWeakSeedPasswords(t *testing.T) {
	t.Setenv("NM_ALLOW_INSECURE", "")
	cfg := Config{
		AuthAdminUser:        "admin",
		AuthAdminPassword:    "admin",
		AuthOperatorUser:     "operator",
		AuthOperatorPassword: "strong-op!",
	}
	w := cfg.SecurityWarnings()
	if len(w) != 1 {
		t.Fatalf("want 1 warning, got %#v", w)
	}
}

func TestValidateSecurityAllowsOverride(t *testing.T) {
	t.Setenv("NM_ALLOW_INSECURE", "1")
	cfg := Config{
		APIAuthToken:  "dev-insecure-change-me",
		SessionSecret: "dev-session-secret-change-me",
	}
	if err := cfg.ValidateSecurity(); err != nil {
		t.Fatalf("NM_ALLOW_INSECURE should permit placeholders: %v", err)
	}
}

func TestValidateSecurityDisabledAuthRequiresInsecure(t *testing.T) {
	t.Setenv("NM_ALLOW_INSECURE", "")
	cfg := Config{
		APIAuthDisabled: true,
		AuthDisabled:    true,
	}
	if err := cfg.ValidateSecurity(); err == nil {
		t.Fatal("expected error when *_DISABLED without NM_ALLOW_INSECURE")
	}

	t.Setenv("NM_ALLOW_INSECURE", "1")
	if err := cfg.ValidateSecurity(); err != nil {
		t.Fatalf("NM_ALLOW_INSECURE should permit *_DISABLED: %v", err)
	}
}

func TestValidateSecurityAPIAuthDisabledAlone(t *testing.T) {
	t.Setenv("NM_ALLOW_INSECURE", "")
	cfg := Config{
		APIAuthDisabled: true,
		SessionSecret:   "unique-session-secret",
	}
	if err := cfg.ValidateSecurity(); err == nil {
		t.Fatal("expected error for API_AUTH_DISABLED without NM_ALLOW_INSECURE")
	}
}

func TestAPIAuthTokensIncludesPrevious(t *testing.T) {
	cfg := Config{APIAuthToken: "current", APIAuthPreviousToken: "previous"}
	got := cfg.APIAuthTokens()
	if len(got) != 2 || got[0] != "current" || got[1] != "previous" {
		t.Fatalf("tokens=%#v", got)
	}
	cfg.APIAuthPreviousToken = "current"
	got = cfg.APIAuthTokens()
	if len(got) != 1 || got[0] != "current" {
		t.Fatalf("dedupe want 1, got %#v", got)
	}
}

func TestValidateSecurityRejectsPreviousPlaceholder(t *testing.T) {
	t.Setenv("NM_ALLOW_INSECURE", "")
	cfg := Config{
		APIAuthToken:         "unique-token-xyz",
		APIAuthPreviousToken: "dev-insecure-change-me",
		SessionSecret:        "ok-secret-not-placeholder",
	}
	if err := cfg.ValidateSecurity(); err == nil {
		t.Fatal("expected error for insecure previous token")
	}
}
