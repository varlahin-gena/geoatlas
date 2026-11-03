package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"geoatlas/internal/adapter/httpapi/loginthrottle"
	"geoatlas/internal/auth"
	usecaseaudit "geoatlas/internal/usecase/auditlog"
)

type securityThreatWindow struct {
	CSRFReject   int64 `json:"csrf_reject"`
	ProxyGate    int64 `json:"proxy_gate"`
	APIRateLimit int64 `json:"api_rate_limit"`
}

type securityThreatCounts struct {
	Last24h securityThreatWindow `json:"last_24h"`
	Last7d  securityThreatWindow `json:"last_7d"`
}

type securityPostureUsers struct {
	Total             int            `json:"total"`
	ByRole            map[string]int `json:"by_role"`
	MustResetPassword int            `json:"must_reset_password"`
}

type securityPostureTokens struct {
	Total    int `json:"total"`
	Expiring int `json:"expiring_soon"` // within 30 days
}

type securityPostureTLS struct {
	Configured bool   `json:"configured"`
	DaysLeft   int    `json:"days_left,omitempty"`
	SelfSigned bool   `json:"self_signed,omitempty"`
	NotAfter   string `json:"not_after,omitempty"`
}

type securityPosture struct {
	Users  securityPostureUsers  `json:"users"`
	Tokens securityPostureTokens `json:"tokens"`
	TLS    securityPostureTLS    `json:"tls"`
}

type securitySummaryResponse struct {
	FailedLogins []loginthrottle.FailedLoginEvent `json:"failed_logins"`
	LoginPolicy  LoginPolicyFile                  `json:"login_policy"`
	Sessions     []auth.SessionRecord             `json:"sessions"`
	AuthRecent   []usecaseaudit.AuditEvent        `json:"auth_recent"`
	ThreatCounts securityThreatCounts             `json:"threat_counts"`
	Posture      securityPosture                  `json:"posture"`
}

var authRecentActions = []string{
	"auth.login.success",
	"auth.login.failed",
	"auth.logout",
	"auth.password.change",
	"auth.password.reset",
	"auth.session.revoke",
	"auth.session.revoke_all",
	"auth.login_policy.update",
}

// GetSecuritySummary — GET /api/system/security
func (h *SystemHandler) GetSecuritySummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := securitySummaryResponse{
		FailedLogins: []loginthrottle.FailedLoginEvent{},
		LoginPolicy:  defaultLoginPolicy(),
		Sessions:     []auth.SessionRecord{},
		AuthRecent:   []usecaseaudit.AuditEvent{},
		Posture: securityPosture{
			Users:  securityPostureUsers{ByRole: map[string]int{}},
			Tokens: securityPostureTokens{},
			TLS:    securityPostureTLS{},
		},
	}

	if h.loginLimiter != nil {
		out.FailedLogins = h.loginLimiter.SnapshotFailures()
		p := h.loginLimiter.Policy()
		out.LoginPolicy = LoginPolicyFile{MaxFails: p.MaxFails, WindowSec: p.WindowSec, LockoutSec: p.LockoutSec}
	}
	if h.sessionRegistry != nil {
		out.Sessions = h.sessionRegistry.List()
	}

	if h.logs != nil && h.logs.Enabled() {
		recent, err := h.logs.ListAudit(ctx, usecaseaudit.AuditQuery{
			Since:   time.Now().UTC().Add(-7 * 24 * time.Hour),
			Limit:   50,
			Actions: authRecentActions,
		})
		if err == nil && recent != nil {
			out.AuthRecent = recent
		}
		out.ThreatCounts.Last24h = threatWindowFromCounts(h.logs, ctx, time.Now().UTC().Add(-24*time.Hour))
		out.ThreatCounts.Last7d = threatWindowFromCounts(h.logs, ctx, time.Now().UTC().Add(-7*24*time.Hour))
	}

	out.Posture = h.buildSecurityPosture()
	writeJSON(w, http.StatusOK, out)
}

func threatWindowFromCounts(logs *usecaseaudit.Service, ctx context.Context, since time.Time) securityThreatWindow {
	var w securityThreatWindow
	if logs == nil {
		return w
	}
	counts, err := logs.CountAuditByAction(ctx, since, "security.")
	if err != nil {
		return w
	}
	for _, c := range counts {
		switch c.Action {
		case "security.csrf.reject":
			w.CSRFReject += c.Count
		case "security.proxy_gate.reject":
			w.ProxyGate += c.Count
		case "security.api_rate_limit":
			w.APIRateLimit += c.Count
		}
	}
	return w
}

func (h *SystemHandler) buildSecurityPosture() securityPosture {
	p := securityPosture{
		Users:  securityPostureUsers{ByRole: map[string]int{}},
		Tokens: securityPostureTokens{},
		TLS:    securityPostureTLS{},
	}
	if lister, ok := h.users.(interface{ List() []auth.UserPublic }); ok {
		for _, u := range lister.List() {
			p.Users.Total++
			p.Users.ByRole[u.Role]++
			if u.MustResetPassword {
				p.Users.MustResetPassword++
			}
		}
	}
	if h.apiTokens != nil {
		now := time.Now().UTC()
		soon := now.Add(30 * 24 * time.Hour)
		for _, t := range h.apiTokens.List() {
			p.Tokens.Total++
			exp := strings.TrimSpace(t.ExpiresAt)
			if exp == "" {
				continue
			}
			ts, err := time.Parse(time.RFC3339, exp)
			if err != nil {
				continue
			}
			if ts.Before(soon) {
				p.Tokens.Expiring++
			}
		}
	}
	if h.tlsUC != nil {
		st, err := h.tlsUC.Status()
		if err == nil {
			// tls.Status.Configured = каталог TLS_CERT_DIR доступен, НЕ «сертификат установлен».
			// В posture для UI — только реальные PEM на диске.
			p.TLS.Configured = st.CertPresent && st.KeyPresent
			if st.Cert != nil {
				p.TLS.DaysLeft = st.Cert.DaysLeft
				p.TLS.SelfSigned = st.Cert.SelfSigned
				p.TLS.NotAfter = st.Cert.NotAfter
			}
		}
	}
	return p
}
