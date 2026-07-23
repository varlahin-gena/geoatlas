package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"network_monitor/internal/auth"
	usecaseauth "network_monitor/internal/usecase/auth"
)

type AuthHandler struct{ *Deps }
type UsersHandler struct{ *Deps }

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authUserResponse struct {
	Username          string `json:"username"`
	FullName          string `json:"full_name,omitempty"`
	Role              string `json:"role"`
	MustResetPassword bool   `json:"must_reset_password,omitempty"`
	AuthDisabled      bool   `json:"authDisabled,omitempty"`
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

func userResponse(u auth.User) authUserResponse {
	return authUserResponse{
		Username:          u.Username,
		FullName:          u.FullName,
		Role:              u.Role,
		MustResetPassword: u.MustResetPassword,
	}
}

func userPublicResponse(u auth.UserPublic) authUserResponse {
	return authUserResponse{
		Username:          u.Username,
		FullName:          u.FullName,
		Role:              u.Role,
		MustResetPassword: u.MustResetPassword,
	}
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
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "username and password required"})
		return
	}

	ip := clientIP(r)
	if !defaultLoginLimiter.allow(ip) {
		defaultLoginLimiter.recordFailure(ip, req.Username)
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many attempts"})
		return
	}

	out, err := h.authUC.Login(req.Username, req.Password)
	if errors.Is(err, usecaseauth.ErrInvalidCredentials) {
		defaultLoginLimiter.recordFailure(ip, req.Username)
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
	defaultLoginLimiter.recordSuccess(ip)

	if h.sessions != nil {
		SetCookie(w, r, out.Token, h.sessions.TTL())
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
		writeJSON(w, http.StatusOK, authUserResponse{
			Username:     "anonymous",
			Role:         auth.RoleAdministrator,
			AuthDisabled: true,
		})
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
	EnsureCSRFCookie(w, r, h.authUC.SessionTTL())
	writeJSON(w, http.StatusOK, userPublicResponse(pub))
}

// ChangePassword — POST /api/auth/change-password (любой залогиненный)
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
	var req changePasswordRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	pub, err := h.authUC.ChangePassword(sess.Username, req.OldPassword, req.NewPassword)
	if err != nil {
		writeUserStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userPublicResponse(pub))
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
	return h == nil || h.Deps == nil || h.cfg.AuthDisabled
}

func (h *AuthHandler) bearerScopeOK(r *http.Request, need string) bool {
	if h == nil || h.Deps == nil || h.cfg.APIAuthDisabled {
		return false
	}
	env := h.cfg.APIAuthTokens()
	ba := newBearerAuth(env, h.apiTokens)
	return ba.ok(r, need)
}

// --- Users CRUD (admin) ---

func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
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

func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	var req createUserRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if req.MustResetPassword == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "must_reset_password is required"})
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
		writeUserStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, pub)
}

func (h *UsersHandler) SetRole(w http.ResponseWriter, r *http.Request) {
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	username := mux.Vars(r)["username"]
	var req setRoleRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	pub, err := h.authUC.SetRole(username, req.Role)
	if err != nil {
		writeUserStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pub)
}

func (h *UsersHandler) SetFullName(w http.ResponseWriter, r *http.Request) {
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	username := mux.Vars(r)["username"]
	var req setFullNameRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	pub, err := h.authUC.SetFullName(username, req.FullName)
	if err != nil {
		writeUserStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pub)
}

func (h *UsersHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	username := mux.Vars(r)["username"]
	var req resetPasswordRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json"})
		return
	}
	if req.MustResetPassword == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "must_reset_password is required"})
		return
	}
	pub, err := h.authUC.ResetPassword(usecaseauth.ResetPasswordInput{
		Username:          username,
		Password:          req.Password,
		MustResetPassword: *req.MustResetPassword,
	})
	if err != nil {
		writeUserStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pub)
}

func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.authUC == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
		return
	}
	username := mux.Vars(r)["username"]
	actor := ""
	if sess, ok := SessionFromContext(r.Context()); ok {
		actor = sess.Username
	} else if sess, err := SessionFromRequest(r, h.sessions); err == nil {
		actor = sess.Username
	}
	if err := h.authUC.DeleteUser(username, actor); err != nil {
		writeUserStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeUserStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrUserExists):
		writeJSON(w, http.StatusConflict, map[string]any{"error": "user already exists"})
	case errors.Is(err, auth.ErrUserNotFound):
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "user not found"})
	case errors.Is(err, auth.ErrInvalidUsername):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid username"})
	case errors.Is(err, auth.ErrInvalidPassword):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid password"})
	case errors.Is(err, auth.ErrInvalidRole):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid role"})
	case errors.Is(err, auth.ErrInvalidFullName):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid full name"})
	case errors.Is(err, auth.ErrLastAdmin):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cannot remove or demote the last administrator"})
	case errors.Is(err, auth.ErrSelfDelete):
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "cannot delete your own account"})
	case errors.Is(err, auth.ErrBadOldPassword):
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "current password is incorrect"})
	case errors.Is(err, usecaseauth.ErrNotConfigured):
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "auth not configured"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal error"})
	}
}
