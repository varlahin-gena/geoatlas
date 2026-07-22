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
