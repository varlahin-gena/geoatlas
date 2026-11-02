package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"geoatlas/internal/fileatomic"
	usecaseaudit "geoatlas/internal/usecase/auditlog"
)

// LoginPolicyFile — persisted lockout thresholds.
type LoginPolicyFile struct {
	MaxFails   int `json:"max_fails"`
	WindowSec  int `json:"window_sec"`
	LockoutSec int `json:"lockout_sec"`
}

const (
	defaultLoginMaxFails   = 10
	defaultLoginWindowSec  = 60
	defaultLoginLockoutSec = 300
)

func defaultLoginPolicy() LoginPolicyFile {
	return LoginPolicyFile{
		MaxFails:   defaultLoginMaxFails,
		WindowSec:  defaultLoginWindowSec,
		LockoutSec: defaultLoginLockoutSec,
	}
}

// LoadLoginPolicy reads JSON; missing file → defaults.
func LoadLoginPolicy(path string) (LoginPolicyFile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return defaultLoginPolicy(), nil
	}
	data, err := fileatomic.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultLoginPolicy(), nil
		}
		return LoginPolicyFile{}, err
	}
	var pol LoginPolicyFile
	if err := json.Unmarshal(data, &pol); err != nil {
		return LoginPolicyFile{}, err
	}
	return normalizeLoginPolicy(pol), nil
}

func saveLoginPolicy(path string, pol LoginPolicyFile) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("login policy path is empty")
	}
	pol = normalizeLoginPolicy(pol)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(pol, "", "  ")
	if err != nil {
		return err
	}
	return fileatomic.WriteFile(path, raw, 0o600)
}

func normalizeLoginPolicy(pol LoginPolicyFile) LoginPolicyFile {
	d := defaultLoginPolicy()
	if pol.MaxFails <= 0 {
		pol.MaxFails = d.MaxFails
	}
	if pol.WindowSec <= 0 {
		pol.WindowSec = d.WindowSec
	}
	if pol.LockoutSec <= 0 {
		pol.LockoutSec = d.LockoutSec
	}
	return pol
}

func validateLoginPolicy(pol LoginPolicyFile) error {
	if pol.MaxFails < 3 || pol.MaxFails > 100 {
		return fmt.Errorf("max_fails must be 3–100")
	}
	if pol.WindowSec < 30 || pol.WindowSec > 86400 {
		return fmt.Errorf("window_sec must be 30–86400")
	}
	if pol.LockoutSec < 30 || pol.LockoutSec > 86400 {
		return fmt.Errorf("lockout_sec must be 30–86400")
	}
	return nil
}

// GetLoginPolicy — GET /api/system/login-policy
func (h *SystemHandler) GetLoginPolicy(w http.ResponseWriter, r *http.Request) {
	pol := defaultLoginPolicy()
	if h.loginLimiter != nil {
		p := h.loginLimiter.Policy()
		pol = LoginPolicyFile{MaxFails: p.MaxFails, WindowSec: p.WindowSec, LockoutSec: p.LockoutSec}
	}
	writeJSON(w, http.StatusOK, pol)
}

// PutLoginPolicy — PUT /api/system/login-policy
func (h *SystemHandler) PutLoginPolicy(w http.ResponseWriter, r *http.Request) {
	var pol LoginPolicyFile
	if !decodeJSONBody(w, r, &pol, defaultJSONBodyLimit) {
		return
	}
	if err := validateLoginPolicy(pol); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	path := h.loginPolicyFile
	if path != "" {
		if err := saveLoginPolicy(path, pol); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to save login policy"})
			return
		}
	}
	if h.loginLimiter != nil {
		h.loginLimiter.SetPolicy(pol.MaxFails, time.Duration(pol.WindowSec)*time.Second, time.Duration(pol.LockoutSec)*time.Second)
	}
	writeAuditEvent(r.Context(), h.logs, usecaseaudit.AuditEvent{
		Actor:        actorFromRequest(r),
		Action:       "auth.login_policy.update",
		ResourceType: "system",
		ResourceID:   "login_policy",
		Result:       "succeeded",
		IP:           clientIPFromRequest(r),
		Details: map[string]any{
			"max_fails":   pol.MaxFails,
			"window_sec":  pol.WindowSec,
			"lockout_sec": pol.LockoutSec,
		},
	})
	writeJSON(w, http.StatusOK, pol)
}
