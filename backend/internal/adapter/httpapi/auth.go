package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"geoatlas/internal/adapter/httpapi/loginthrottle"
	"geoatlas/internal/auth"
	usecaseaudit "geoatlas/internal/usecase/auditlog"
	usecaseauth "geoatlas/internal/usecase/auth"
)

type AuthHandler struct{ *AuthDeps }
type UsersHandler struct{ *AuthDeps }

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authUserResponse struct {
	Username           string `json:"username"`
	FullName           string `json:"full_name,omitempty"`
	Role               string `json:"role"`
	MustResetPassword  bool   `json:"must_reset_password,omitempty"`
	GeoWizardDismissed bool   `json:"geo_wizard_dismissed,omitempty"`
	AuthDisabled       bool   `json:"authDisabled,omitempty"`
	ReputationEnabled  bool   `json:"reputationEnabled"`
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type createUserRequest struct {
	Username          string `json:"username"`
	FullName          string `json:"full_name"`
	Password          string `json:"password"`
	Role              string `json:"role"`
	MustResetPassword *bool  `json:"must_reset_password"` // обязательно в запросе
}

type resetPasswordRequest struct {
	Password          string `json:"password"`
	MustResetPassword *bool  `json:"must_reset_password"`
}

type setRoleRequest struct {
	Role string `json:"role"`
}

type setFullNameRequest struct {
	FullName string `json:"full_name"`
}

type geoWizardDismissRequest struct {
	Dismissed *bool `json:"dismissed"`
}

func userResponse(u auth.User) authUserResponse {
	return authUserResponse{
		Username:           u.Username,
		FullName:           u.FullName,
		Role:               u.Role,
		MustResetPassword:  u.MustResetPassword,
		GeoWizardDismissed: u.GeoWizardDismissed,
	}
}

func userPublicResponse(u auth.UserPublic) authUserResponse {
	return authUserResponse{
		Username:           u.Username,
		FullName:           u.FullName,
		Role:               u.Role,
		MustResetPassword:  u.MustResetPassword,
		GeoWizardDismissed: u.GeoWizardDismissed,
	}
}

func (h *AuthHandler) withModuleFlags(resp authUserResponse) authUserResponse {
	if h != nil && h.AuthDeps != nil {
		resp.ReputationEnabled = h.cfg.ReputationFetchEnabled
	}
	return resp
}

// Login — POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if h.authDisabled() {
		writeJSON(w, http.StatusOK, authUserResponse{
			Username:     "anonymous",
			Role:         auth.RoleAdministrator,
			AuthDisabled: true,
		})
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}

	var req loginRequest
	if !decodeJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeBadRequest(w, "username and password required")
		return
	}

	ip := loginthrottle.ClientIP(r)
	lim := h.loginLimiter
	if lim == nil {
		lim = loginthrottle.New(10, time.Minute, 5*time.Minute)
	}
	if !lim.Allow(ip) {
		lim.RecordFailure(ip, req.Username)
		writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
			Actor:        req.Username,
			Action:       "auth.login.failed",
			ResourceType: "session",
			ResourceID:   req.Username,
			Result:       "failed",
			IP:           ip,
			Details:      map[string]any{"reason": "rate_limited"},
		})
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many attempts"})
		return
	}

	out, err := h.authUC.Login(req.Username, req.Password)
	if errors.Is(err, usecaseauth.ErrInvalidCredentials) {
		lim.RecordFailure(ip, req.Username)
		writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
			Actor:        req.Username,
			Action:       "auth.login.failed",
			ResourceType: "session",
			ResourceID:   req.Username,
			Result:       "failed",
			IP:           ip,
			Details:      map[string]any{"reason": "invalid_credentials"},
		})
		time.Sleep(200 * time.Millisecond)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid credentials"})
		return
	}
	if errors.Is(err, usecaseauth.ErrNotConfigured) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "session error"})
		return
	}
	lim.RecordSuccess(ip)
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor:        out.User.Username,
		Action:       "auth.login.success",
		ResourceType: "session",
		ResourceID:   out.User.Username,
		Result:       "succeeded",
		IP:           ip,
		Details:      map[string]any{"role": out.User.Role},
	})

	if h.sessions != nil {
		SetCookie(w, r, out.Token, auth.CookieTTLForRole(out.User.Role, h.sessions.TTL()))
	}
	writeJSON(w, http.StatusOK, userResponse(out.User))
}

// Logout — POST /api/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ClearCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Me — GET /api/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	if h.authDisabled() {
		writeJSON(w, http.StatusOK, h.withModuleFlags(authUserResponse{
			Username:     "anonymous",
			Role:         auth.RoleAdministrator,
			AuthDisabled: true,
		}))
		return
	}
	sess, ok := SessionFromContext(r.Context())
	if !ok {
		var err error
		sess, err = SessionFromRequest(r, h.sessions)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
	}
	if live, ok := auth.LiveSession(h.users, sess); ok {
		sess = live
	} else {
		ClearCookie(w, r)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	pub, err := h.authUC.Me(sess.Username)
	if errors.Is(err, usecaseauth.ErrUnauthorized) {
		ClearCookie(w, r)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	EnsureCSRFCookie(w, r, auth.CookieTTLForRole(sess.Role, h.authUC.SessionTTL()))
	writeJSON(w, http.StatusOK, h.withModuleFlags(userPublicResponse(pub)))
}

// ChangePassword — POST /api/auth/change-password (любой залогиненный).
// Bump session_version + выдаёт новый cookie (остальные сессии revoke).
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if h.authDisabled() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	sess, err := SessionFromRequest(r, h.sessions)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if _, ok := auth.LiveSession(h.users, sess); !ok {
		ClearCookie(w, r)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	var req changePasswordRequest
	if !decodeJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	out, err := h.authUC.ChangePassword(sess.Username, req.OldPassword, req.NewPassword)
	if err != nil {
		writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
			Actor:        sess.Username,
			Action:       "auth.password.change",
			ResourceType: "user",
			ResourceID:   sess.Username,
			Result:       "failed",
			IP:           clientIPFromRequest(r),
			Details:      map[string]any{"error": err.Error()},
		})
		writeUserStoreError(w, err)
		return
	}
	if h.sessions != nil {
		SetCookie(w, r, out.Token, auth.CookieTTLForRole(out.User.Role, h.sessions.TTL()))
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor:        sess.Username,
		Action:       "auth.password.change",
		ResourceType: "user",
		ResourceID:   sess.Username,
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
	})
	writeJSON(w, http.StatusOK, userPublicResponse(out.User))
}

// LogoutAll — POST /api/auth/logout-all: revoke всех сессий пользователя + clear cookie.
func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	if h.authDisabled() {
		ClearCookie(w, r)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	sess, err := SessionFromRequest(r, h.sessions)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	live, ok := auth.LiveSession(h.users, sess)
	if !ok {
		ClearCookie(w, r)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	if err := h.authUC.LogoutAll(live.Username); err != nil {
		writeUserStoreError(w, err)
		return
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor:        live.Username,
		Action:       "auth.session.revoke_all",
		ResourceType: "user",
		ResourceID:   live.Username,
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
	})
	ClearCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DismissGeoWizard — POST /api/auth/geo-wizard-dismiss (свой флаг first-run GeoIP).
func (h *AuthHandler) DismissGeoWizard(w http.ResponseWriter, r *http.Request) {
	if h.authDisabled() {
		writeJSON(w, http.StatusOK, h.withModuleFlags(authUserResponse{
			Username:           "anonymous",
			Role:               auth.RoleAdministrator,
			AuthDisabled:       true,
			GeoWizardDismissed: true,
		}))
		return
	}
	sess, err := SessionFromRequest(r, h.sessions)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}
	var req geoWizardDismissRequest
	if !decodeJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}
	if req.Dismissed == nil {
		writeBadRequest(w, "dismissed required")
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	pub, err := h.authUC.SetGeoWizardDismissed(sess.Username, *req.Dismissed)
	if err != nil {
		writeUserStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.withModuleFlags(userPublicResponse(pub)))
}

// Check — GET /api/auth/check (nginx auth_request: любой залогиненный или Bearer≥read).
func (h *AuthHandler) Check(w http.ResponseWriter, r *http.Request) {
	if h.authDisabled() {
		w.WriteHeader(http.StatusOK)
		return
	}
	if h.bearerScopeOK(r, auth.ScopeRead) {
		w.WriteHeader(http.StatusOK)
		return
	}
	sess, err := SessionFromRequest(r, h.sessions)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if _, ok := auth.LiveSession(h.users, sess); !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// CheckOps — GET /api/auth/check-ops (nginx: Bearer≥ops / administrator / API_AUTH_DISABLED).
func (h *AuthHandler) CheckOps(w http.ResponseWriter, r *http.Request) {
	if h.authDisabled() {
		w.WriteHeader(http.StatusOK)
		return
	}
	if h.cfg.APIAuthDisabled {
		w.WriteHeader(http.StatusOK)
		return
	}
	if h.bearerScopeOK(r, auth.ScopeOps) {
		w.WriteHeader(http.StatusOK)
		return
	}
	sess, err := SessionFromRequest(r, h.sessions)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	live, ok := auth.LiveSession(h.users, sess)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !auth.IsAdmin(live.Role) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// CheckAdmin — GET /api/auth/check-admin (nginx: administrator / Bearer admin / API_AUTH_DISABLED).
func (h *AuthHandler) CheckAdmin(w http.ResponseWriter, r *http.Request) {
	if h.authDisabled() {
		w.WriteHeader(http.StatusOK)
		return
	}
	// Совпадает с requireOpsMW: при API_AUTH_DISABLED ops-эндпоинты открыты.
	// Для HTML admin-страниц тоже открываем при API_AUTH_DISABLED (как раньше).
	if h.cfg.APIAuthDisabled {
		w.WriteHeader(http.StatusOK)
		return
	}
	if h.bearerScopeOK(r, auth.ScopeAdmin) {
		w.WriteHeader(http.StatusOK)
		return
	}
	sess, err := SessionFromRequest(r, h.sessions)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	live, ok := auth.LiveSession(h.users, sess)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if !auth.IsAdmin(live.Role) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *AuthHandler) authDisabled() bool {
	return h == nil || h.AuthDeps == nil || h.cfg.AuthDisabled
}

func (h *AuthHandler) bearerScopeOK(r *http.Request, need string) bool {
	if h == nil || h.AuthDeps == nil || h.cfg.APIAuthDisabled {
		return false
	}
	env := h.cfg.APIAuthTokens()
	ba := newBearerAuth(env, h.cfg.APIOpsTokens(), h.apiTokens)
	return ba.OK(r, need)
}

// --- Users CRUD (admin) ---

func (h *UsersHandler) authModuleDisabled(w http.ResponseWriter) bool {
	if h != nil && h.AuthDeps != nil && h.cfg.AuthDisabled {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth module disabled"})
		return true
	}
	return false
}

func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.authModuleDisabled(w) {
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	users, err := h.authUC.ListUsers()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

// Directory — краткий список УЗ для назначения алертов (login, без ролей/паролей).
func (h *UsersHandler) Directory(w http.ResponseWriter, r *http.Request) {
	if h != nil && h.AuthDeps != nil && h.cfg.AuthDisabled {
		writeJSON(w, http.StatusOK, map[string]any{"users": []any{}})
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	users, err := h.authUC.ListUsers()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	out := make([]map[string]string, 0, len(users))
	for _, u := range users {
		item := map[string]string{"username": u.Username}
		if fn := strings.TrimSpace(u.FullName); fn != "" {
			item["full_name"] = fn
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h.authModuleDisabled(w) {
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	var req createUserRequest
	if !decodeJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}
	if req.MustResetPassword == nil {
		writeBadRequest(w, "must_reset_password is required")
		return
	}
	pub, err := h.authUC.CreateUser(usecaseauth.CreateUserInput{
		Username:          req.Username,
		FullName:          req.FullName,
		Password:          req.Password,
		Role:              req.Role,
		MustResetPassword: *req.MustResetPassword,
	})
	if err != nil {
		writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
			Actor:        actorFromRequest(r),
			Action:       "auth.user.create",
			ResourceType: "user",
			ResourceID:   req.Username,
			Result:       "failed",
			IP:           clientIPFromRequest(r),
			Details:      map[string]any{"error": err.Error()},
		})
		writeUserStoreError(w, err)
		return
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor:        actorFromRequest(r),
		Action:       "auth.user.create",
		ResourceType: "user",
		ResourceID:   pub.Username,
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
		Details:      map[string]any{"role": pub.Role, "must_reset_password": pub.MustResetPassword},
	})
	writeJSON(w, http.StatusCreated, pub)
}

func (h *UsersHandler) SetRole(w http.ResponseWriter, r *http.Request) {
	if h.authModuleDisabled(w) {
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	username := r.PathValue("username")
	var req setRoleRequest
	if !decodeJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}
	pub, err := h.authUC.SetRole(username, req.Role)
	if err != nil {
		writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
			Actor:        actorFromRequest(r),
			Action:       "auth.user.role_change",
			ResourceType: "user",
			ResourceID:   username,
			Result:       "failed",
			IP:           clientIPFromRequest(r),
			Details:      map[string]any{"error": err.Error(), "role": req.Role},
		})
		writeUserStoreError(w, err)
		return
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor:        actorFromRequest(r),
		Action:       "auth.user.role_change",
		ResourceType: "user",
		ResourceID:   pub.Username,
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
		Details:      map[string]any{"role": pub.Role},
	})
	writeJSON(w, http.StatusOK, pub)
}

func (h *UsersHandler) SetFullName(w http.ResponseWriter, r *http.Request) {
	if h.authModuleDisabled(w) {
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	username := r.PathValue("username")
	var req setFullNameRequest
	if !decodeJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}
	pub, err := h.authUC.SetFullName(username, req.FullName)
	if err != nil {
		writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
			Actor:        actorFromRequest(r),
			Action:       "auth.user.update",
			ResourceType: "user",
			ResourceID:   username,
			Result:       "failed",
			IP:           clientIPFromRequest(r),
			Details:      map[string]any{"error": err.Error()},
		})
		writeUserStoreError(w, err)
		return
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor:        actorFromRequest(r),
		Action:       "auth.user.update",
		ResourceType: "user",
		ResourceID:   pub.Username,
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
	})
	writeJSON(w, http.StatusOK, pub)
}

func (h *UsersHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if h.authModuleDisabled(w) {
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	username := r.PathValue("username")
	var req resetPasswordRequest
	if !decodeJSONBody(w, r, &req, defaultJSONBodyLimit) {
		return
	}
	if req.MustResetPassword == nil {
		writeBadRequest(w, "must_reset_password is required")
		return
	}
	pub, err := h.authUC.ResetPassword(usecaseauth.ResetPasswordInput{
		Username:          username,
		Password:          req.Password,
		MustResetPassword: *req.MustResetPassword,
	})
	if err != nil {
		writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
			Actor:        actorFromRequest(r),
			Action:       "auth.password.reset",
			ResourceType: "user",
			ResourceID:   username,
			Result:       "failed",
			IP:           clientIPFromRequest(r),
			Details:      map[string]any{"error": err.Error()},
		})
		writeUserStoreError(w, err)
		return
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor:        actorFromRequest(r),
		Action:       "auth.password.reset",
		ResourceType: "user",
		ResourceID:   pub.Username,
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
		Details:      map[string]any{"must_reset_password": pub.MustResetPassword},
	})
	writeJSON(w, http.StatusOK, pub)
}

func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.authModuleDisabled(w) {
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	username := r.PathValue("username")
	actor := ""
	if sess, ok := SessionFromContext(r.Context()); ok {
		actor = sess.Username
	} else if sess, err := SessionFromRequest(r, h.sessions); err == nil {
		actor = sess.Username
	}
	if err := h.authUC.DeleteUser(username, actor); err != nil {
		writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
			Actor:        actorFromRequest(r),
			Action:       "auth.user.delete",
			ResourceType: "user",
			ResourceID:   username,
			Result:       "failed",
			IP:           clientIPFromRequest(r),
			Details:      map[string]any{"error": err.Error()},
		})
		writeUserStoreError(w, err)
		return
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor:        actorFromRequest(r),
		Action:       "auth.user.delete",
		ResourceType: "user",
		ResourceID:   username,
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeUserStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrUserExists):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "user already exists"})
	case errors.Is(err, auth.ErrUserNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
	case errors.Is(err, auth.ErrInvalidUsername):
		writeBadRequest(w, "invalid username")
	case errors.Is(err, auth.ErrInvalidPassword):
		writeBadRequest(w, "invalid password")
	case errors.Is(err, auth.ErrInvalidRole):
		writeBadRequest(w, "invalid role")
	case errors.Is(err, auth.ErrInvalidFullName):
		writeBadRequest(w, "invalid full name")
	case errors.Is(err, auth.ErrLastAdmin):
		writeBadRequest(w, "cannot remove or demote the last administrator")
	case errors.Is(err, auth.ErrSelfDelete):
		writeBadRequest(w, "cannot delete your own account")
	case errors.Is(err, auth.ErrBadOldPassword):
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "current password is incorrect"})
	case errors.Is(err, usecaseauth.ErrNotConfigured):
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal error"})
	}
}
