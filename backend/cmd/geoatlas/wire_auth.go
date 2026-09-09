package main

import (
	"fmt"
	"log/slog"
	"time"

	"geoatlas/internal/auth"
	"geoatlas/internal/config"
)

type authParts struct {
	users     *auth.UserStore
	sessions  *auth.SessionManager
	apiTokens *auth.TokenStore
}

func buildAuth(cfg config.Config) (authParts, error) {
	var out authParts
	if cfg.Auth.Disabled {
		slog.Warn("UI auth disabled — login and role checks are off")
	} else {
		seed, err := auth.SeedUsersFromEnv(
			cfg.Auth.AdminUser, cfg.Auth.AdminPassword,
			cfg.Auth.OperatorUser, cfg.Auth.OperatorPassword,
			cfg.Auth.AdminMustReset,
		)
		if err != nil {
			return out, fmt.Errorf("auth seed: %w", err)
		}
		users, err := auth.OpenOrSeed(cfg.Auth.UsersFile, seed)
		if err != nil {
			return out, fmt.Errorf("auth users file %q: %w", cfg.Auth.UsersFile, err)
		}
		ttl := time.Duration(cfg.Auth.SessionTTLHours) * time.Hour
		sessions, err := auth.NewSessionManager(cfg.Auth.SessionSecret, ttl)
		if err != nil {
			return out, fmt.Errorf("session manager: %w", err)
		}
		out.users = users
		out.sessions = sessions
		slog.Info("UI auth enabled",
			"users", users.Len(),
			"users_file", cfg.Auth.UsersFile,
			"session_ttl", ttl.String(),
		)
	}
	if !cfg.Auth.APIAuthDisabled {
		apiTokens, err := auth.OpenOrCreateTokenStore(cfg.Auth.APITokensFile)
		if err != nil {
			return out, fmt.Errorf("api tokens file %q: %w", cfg.Auth.APITokensFile, err)
		}
		out.apiTokens = apiTokens
		slog.Info("API token store ready", "tokens", apiTokens.Len(), "file", cfg.Auth.APITokensFile)
	}
	return out, nil
}
