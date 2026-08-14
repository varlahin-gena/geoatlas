package main

import (
	"fmt"
	"log/slog"
	"time"

	"network_monitor/internal/auth"
	"network_monitor/internal/config"
)

type authParts struct {
	users     *auth.UserStore
	sessions  *auth.SessionManager
	apiTokens *auth.TokenStore
}

func buildAuth(cfg config.Config) (authParts, error) {
	var out authParts
	if cfg.AuthDisabled {
		slog.Warn("UI auth disabled — login and role checks are off")
	} else {
		seed, err := auth.SeedUsersFromEnv(
			cfg.AuthAdminUser, cfg.AuthAdminPassword,
			cfg.AuthOperatorUser, cfg.AuthOperatorPassword,
			cfg.AuthAdminMustReset,
		)
		if err != nil {
			return out, fmt.Errorf("auth seed: %w", err)
		}
		users, err := auth.OpenOrSeed(cfg.AuthUsersFile, seed)
		if err != nil {
			return out, fmt.Errorf("auth users file %q: %w", cfg.AuthUsersFile, err)
		}
		ttl := time.Duration(cfg.SessionTTLHours) * time.Hour
		sessions, err := auth.NewSessionManager(cfg.SessionSecret, ttl)
		if err != nil {
			return out, fmt.Errorf("session manager: %w", err)
		}
		out.users = users
		out.sessions = sessions
		slog.Info("UI auth enabled",
			"users", users.Len(),
			"users_file", cfg.AuthUsersFile,
			"session_ttl", ttl.String(),
		)
	}
	if !cfg.APIAuthDisabled {
		apiTokens, err := auth.OpenOrCreateTokenStore(cfg.APITokensFile)
		if err != nil {
			return out, fmt.Errorf("api tokens file %q: %w", cfg.APITokensFile, err)
		}
		out.apiTokens = apiTokens
		slog.Info("API token store ready", "tokens", apiTokens.Len(), "file", cfg.APITokensFile)
	}
	return out, nil
}
