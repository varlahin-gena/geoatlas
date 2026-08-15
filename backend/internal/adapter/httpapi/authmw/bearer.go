package authmw

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"network_monitor/internal/auth"
)

// BearerAuth — env Bearer (всегда admin) + именованные токены из TokenStore.
type BearerAuth struct {
	envTokens []string
	store     TokenStore
}

func NewBearerAuth(envTokens []string, store TokenStore) BearerAuth {
	return BearerAuth{envTokens: envTokens, store: store}
}

func (b BearerAuth) Scope(r *http.Request) (string, bool) {
	got := bearerPlain(r)
	if got == "" {
		return "", false
	}
	gotb := []byte(got)
	for _, token := range b.envTokens {
		if token == "" {
			continue
		}
		tb := []byte(token)
		if len(tb) != len(gotb) {
			continue
		}
		if subtle.ConstantTimeCompare(gotb, tb) == 1 {
			return auth.ScopeAdmin, true
		}
	}
	if b.store != nil {
		if scope, ok := b.store.Verify(got); ok {
			return scope, true
		}
	}
	return "", false
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
