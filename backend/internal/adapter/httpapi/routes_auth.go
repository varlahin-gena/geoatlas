package httpapi

import "net/http"

func (w *routeWiring) registerAuthRoutes() {
	// --- Auth (публичные / собственные) ---
	w.handle("POST", "/api/auth/login", chain(http.HandlerFunc(w.auth.Login), maxBytesMW(64<<10)))
	w.handle("POST", "/api/auth/logout",
		chain(http.HandlerFunc(w.auth.Logout), w.csrf),
	)
	w.handle("POST", "/api/auth/logout-all",
		chain(http.HandlerFunc(w.auth.LogoutAll), w.csrf),
	)
	w.handle("GET", "/api/auth/me", http.HandlerFunc(w.auth.Me))
	w.handle("POST", "/api/auth/change-password",
		chain(http.HandlerFunc(w.auth.ChangePassword), w.csrf, maxBytesMW(64<<10)),
	)
	w.handle("POST", "/api/auth/geo-wizard-dismiss",
		chain(http.HandlerFunc(w.auth.DismissGeoWizard), w.loginMW, w.csrf, maxBytesMW(64<<10)),
	)
	w.handle("GET", "/api/auth/check", http.HandlerFunc(w.auth.Check))
	w.handle("GET", "/api/auth/check-ops", http.HandlerFunc(w.auth.CheckOps))
	w.handle("GET", "/api/auth/check-admin", http.HandlerFunc(w.auth.CheckAdmin))
}

func (w *routeWiring) registerUserRoutes() {
	// --- Управление УЗ (только администратор) ---
	w.handle("GET", "/api/users",
		withTimeout(chain(http.HandlerFunc(w.users.List), w.adminMW), healthTimeout),
	)
	w.handle("GET", "/api/users/directory",
		withTimeout(chain(http.HandlerFunc(w.users.Directory), w.loginMW), healthTimeout),
	)
	w.handle("POST", "/api/users",
		chain(http.HandlerFunc(w.users.Create), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("POST", "/api/users/{username}/role",
		chain(http.HandlerFunc(w.users.SetRole), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("POST", "/api/users/{username}/full-name",
		chain(http.HandlerFunc(w.users.SetFullName), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("POST", "/api/users/{username}/reset-password",
		chain(http.HandlerFunc(w.users.ResetPassword), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("DELETE", "/api/users/{username}",
		chain(http.HandlerFunc(w.users.Delete), w.adminMW, w.csrf),
	)
}

func (w *routeWiring) registerTokenRoutes() {
	// --- API-токены (administrator) ---
	w.handle("GET", "/api/tokens",
		withTimeout(chain(http.HandlerFunc(w.tokens.List), w.adminMW), healthTimeout),
	)
	w.handle("POST", "/api/tokens",
		chain(http.HandlerFunc(w.tokens.Create), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("POST", "/api/tokens/{id}/rotate",
		chain(http.HandlerFunc(w.tokens.Rotate), w.adminMW, w.csrf, maxBytesMW(maxJSONBodySize)),
	)
	w.handle("DELETE", "/api/tokens/{id}",
		chain(http.HandlerFunc(w.tokens.Revoke), w.adminMW, w.csrf),
	)
}
