package authmw

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"network_monitor/internal/auth"
)

// BearerAuth — env admin Bearer + optional env ops Bearer + именованные токены из TokenStore.
type BearerAuth struct {
	envAdminTokens []string
	envOpsTokens   []string
	store          TokenStore
}

// NewBearerAuth создаёт проверку Bearer.
// envAdminTokens — API_AUTH_TOKEN(+previous), всегда scope=admin.
// envOpsTokens — API_OPS_TOKEN(+previous), scope=ops (sidecars вроде stats-collector).
func NewBearerAuth(envAdminTokens, envOpsTokens []string, store TokenStore) BearerAuth {
	return BearerAuth{
		envAdminTokens: envAdminTokens,
		envOpsTokens:   envOpsTokens,
		store:          store,
	}
}

func (b BearerAuth) Scope(r *http.Request) (string, bool) {
	got := bearerPlain(r)
	if got == "" {
		return "", false
	}
	gotb := []byte(got)
	if matchEnvToken(gotb, b.envAdminTokens) {
		return auth.ScopeAdmin, true
	}
	if matchEnvToken(gotb, b.envOpsTokens) {
		return auth.ScopeOps, true
	}
	if b.store != nil {
		if scope, ok := b.store.Verify(got); ok {
			return scope, true
		}
	}
	return "", false
}

func matchEnvToken(gotb []byte, tokens []string) bool {
	for _, token := range tokens {
		if token == "" {
			continue
		}
		tb := []byte(token)
		if len(tb) != len(gotb) {
			continue
		}
		if subtle.ConstantTimeCompare(gotb, tb) == 1 {
			return true
		}
	}
	return false
}

func (b BearerAuth) OK(r *http.Request, need string) bool {
	scope, ok := b.Scope(r)
	return ok && auth.ScopeAtLeast(scope, need)
}

func (b BearerAuth) Any(r *http.Request) bool {
	_, ok := b.Scope(r)
	return ok
}

func bearerPlain(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
}
