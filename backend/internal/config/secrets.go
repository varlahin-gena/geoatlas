package config

import (
	"fmt"
	"strings"
)

// minSecretLen — минимальная длина production-секретов (start.sh: openssl rand -hex 32).
const minSecretLen = 16

// Legacy insecure placeholders (старые дефолты compose / ручная подстановка).
var (
	insecureAPIAuthTokens = []string{
		"dev-insecure-change-me",
	}
	insecureSessionSecrets = []string{
		"dev-session-secret-change-me",
	}
)

// ValidateSecurity проверяет секреты перед стартом.
// Legacy placeholders, короткие секреты и флаги *_DISABLED запрещены без NM_ALLOW_INSECURE=1.
// Слабые seed-пароли (литерал admin) не блокируют старт — у них must_reset_password;
// см. SecurityWarnings.
func (c Config) ValidateSecurity() error {
	allowInsecure := envBool("NM_ALLOW_INSECURE", false)

	if (c.APIAuthDisabled || c.AuthDisabled) && !allowInsecure {
		which := "AUTH_DISABLED"
		if c.APIAuthDisabled && c.AuthDisabled {
			which = "AUTH_DISABLED and API_AUTH_DISABLED"
		} else if c.APIAuthDisabled {
			which = "API_AUTH_DISABLED"
		}
		return fmt.Errorf("%s requires NM_ALLOW_INSECURE=1 (local/dev only)", which)
	}

	if !c.APIAuthDisabled {
		token := strings.TrimSpace(c.APIAuthToken)
		if token == "" {
			return fmt.Errorf("API_AUTH_TOKEN is required; set API_AUTH_DISABLED=1 only for local/dev (with NM_ALLOW_INSECURE=1)")
		}
		if !allowInsecure && isListed(token, insecureAPIAuthTokens) {
			return fmt.Errorf("API_AUTH_TOKEN is a known insecure legacy placeholder; generate via start.sh or set a unique token (NM_ALLOW_INSECURE=1 to override)")
		}
		if err := requireMinSecretLen("API_AUTH_TOKEN", token, allowInsecure); err != nil {
			return err
		}
		prev := strings.TrimSpace(c.APIAuthPreviousToken)
		if prev != "" {
			if !allowInsecure && isListed(prev, insecureAPIAuthTokens) {
				return fmt.Errorf("API_AUTH_PREVIOUS_TOKEN is a known insecure placeholder; use a real previous token or unset it")
			}
			if err := requireMinSecretLen("API_AUTH_PREVIOUS_TOKEN", prev, allowInsecure); err != nil {
				return err
			}
		}
	}

	if !c.AuthDisabled {
		secret := strings.TrimSpace(c.SessionSecret)
		if secret == "" {
			return fmt.Errorf("SESSION_SECRET is required when AUTH_DISABLED is not set")
		}
		if !allowInsecure && isListed(secret, insecureSessionSecrets) {
			return fmt.Errorf("SESSION_SECRET is a known insecure legacy placeholder; generate via start.sh (NM_ALLOW_INSECURE=1 to override)")
		}
		if err := requireMinSecretLen("SESSION_SECRET", secret, allowInsecure); err != nil {
			return err
		}
	}

	ingestSecret := strings.TrimSpace(c.IngestSharedSecret)
	if ingestSecret == "" && !allowInsecure {
		return fmt.Errorf("INGEST_SHARED_SECRET is required; generate via start.sh (NM_ALLOW_INSECURE=1 to override for local/dev)")
	}
	if ingestSecret != "" {
		if err := requireMinSecretLen("INGEST_SHARED_SECRET", ingestSecret, allowInsecure); err != nil {
			return err
		}
	}

	chPass := strings.TrimSpace(c.ClickHousePassword)
	if chPass == "" && !allowInsecure {
		return fmt.Errorf("CLICKHOUSE_PASSWORD is required; generate via start.sh (NM_ALLOW_INSECURE=1 to override for local/dev)")
	}
	if chPass != "" {
		if err := requireMinSecretLen("CLICKHOUSE_PASSWORD", chPass, allowInsecure); err != nil {
			return err
		}
	}

	ops := strings.TrimSpace(c.APIOpsToken)
	if ops != "" {
		if err := requireMinSecretLen("API_OPS_TOKEN", ops, allowInsecure); err != nil {
			return err
		}
		admin := strings.TrimSpace(c.APIAuthToken)
		if admin != "" && ops == admin {
			return fmt.Errorf("API_OPS_TOKEN must differ from API_AUTH_TOKEN (ops is for sidecars; admin must stay separate)")
		}
	}
	opsPrev := strings.TrimSpace(c.APIOpsPreviousToken)
	if opsPrev != "" {
		if err := requireMinSecretLen("API_OPS_PREVIOUS_TOKEN", opsPrev, allowInsecure); err != nil {
			return err
		}
	}

	return nil
}

func requireMinSecretLen(name, value string, allowInsecure bool) error {
	if allowInsecure || len(value) >= minSecretLen {
		return nil
	}
	return fmt.Errorf("%s must be at least %d characters (got %d); generate via start.sh (NM_ALLOW_INSECURE=1 to override)", name, minSecretLen, len(value))
}

// SecurityWarnings — нефатальные замечания (слабые seed-пароли).
func (c Config) SecurityWarnings() []string {
	if c.AuthDisabled || envBool("NM_ALLOW_INSECURE", false) {
		return nil
	}
	var out []string
	if strings.TrimSpace(c.AuthAdminPassword) != "" && isWeakSeedPassword(c.AuthAdminUser, c.AuthAdminPassword) {
		out = append(out, "AUTH_ADMIN_PASSWORD is a weak default — change after first login (must_reset_password)")
	}
	if strings.TrimSpace(c.AuthOperatorPassword) != "" && isWeakSeedPassword(c.AuthOperatorUser, c.AuthOperatorPassword) {
		out = append(out, "AUTH_OPERATOR_PASSWORD is a weak default — change after first login (must_reset_password)")
	}
	return out
}

func isWeakSeedPassword(user, pass string) bool {
	u := strings.ToLower(strings.TrimSpace(user))
	p := strings.ToLower(strings.TrimSpace(pass))
	if p == "" {
		return false
	}
	if p == u {
		return true
	}
	switch p {
	case "admin", "operator", "password", "password1", "password12", "password123",
		"changeme", "123456", "12345678", "1234567890", "qwerty", "qwerty123",
		"letmein", "welcome", "welcome1", "admin123", "administrator",
		"passw0rd", "p@ssw0rd", "secret", "default":
		return true
	}
	return false
}

func isListed(v string, list []string) bool {
	for _, item := range list {
		if strings.EqualFold(v, item) {
			return true
		}
	}
	return false
}
