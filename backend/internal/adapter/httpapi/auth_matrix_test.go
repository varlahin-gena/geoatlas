package httpapi

import (
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"network_monitor/internal/config"
)

// authTier documents the middleware wrapping each route in NewServer.
// Keep in sync with server.go and openapi.yaml / README «HTTP API».
type authTier string

const (
	tierPublic authTier = "public" // no session
	tierLogin  authTier = "login"  // any logged-in role
	tierAdmin  authTier = "admin"  // administrator or Bearer
	tierOps    authTier = "ops"    // administrator or Bearer (or AUTH/API_AUTH disabled)
)

func expectedAuthMatrix() map[string]authTier {
	return map[string]authTier{
		"GET /health":                               tierPublic,
		"GET /api/health":                           tierPublic,
		"POST /api/auth/login":                      tierPublic,
		"POST /api/auth/logout":                     tierPublic,
		"GET /api/auth/me":                          tierPublic, // handler returns 401 itself
		"POST /api/auth/change-password":            tierPublic, // handler checks session
		"GET /api/auth/check":                       tierPublic,
		"GET /api/auth/check-admin":                 tierPublic,
		"GET /api/auth/check-ops":                   tierPublic,
		"GET /api/events":                           tierLogin,
		"GET /api/events/series":                    tierLogin,
		"GET /api/system/status":                    tierLogin,
		"GET /api/system/version":                   tierLogin,
		"GET /api/geo-missing":                      tierAdmin,
		"GET /api/geo-ranges/export":                tierOps,
		"POST /api/geo-ranges/clear":                tierAdmin,
		"GET /api/geo-ranges":                       tierAdmin,
		"POST /api/geo-ranges":                      tierOps,
		"PUT /api/geo-ranges":                       tierOps,
		"GET /api/ingest/stats":                     tierOps,
		"POST /api/ingest":                          tierOps,
		"POST /upload-logs":                         tierOps,
		"POST /upload-geo":                          tierOps,
		"POST /upload-reputation":                   tierOps,
		"GET /api/reputation/lists":                 tierAdmin,
		"DELETE /api/reputation/lists/{name}":       tierOps,
		"GET /api/reputation/feeds":                 tierAdmin,
		"POST /api/reputation/feeds":                tierOps,
		"DELETE /api/reputation/feeds/{name}":       tierOps,
		"GET /api/reputation/catalog":               tierAdmin,
		"POST /api/reputation/refresh":              tierOps,
		"GET /api/reputation/lookup":                tierLogin,
		"GET /api/system/stats":                     tierAdmin,
		"GET /api/system/history":                   tierAdmin,
		"GET /api/system/edges-agg":                 tierAdmin,
		"POST /api/system/maintenance/backfill":     tierAdmin,
		"GET /api/system/install-profile":           tierAdmin,
		"GET /api/system/retention":                 tierAdmin,
		"PUT /api/system/retention":                 tierAdmin,
		"GET /api/system/backups":                   tierAdmin,
		"POST /api/system/backups":                  tierAdmin,
		"POST /api/system/backups/{name}/attach":    tierAdmin,
		"POST /api/system/backups/{name}/detach":    tierAdmin,
		"DELETE /api/system/backups/{name}":         tierAdmin,
		"GET /api/parse-errors":                     tierAdmin,
		"GET /api/parse-samples":                    tierAdmin,
		"POST /api/parse-test":                      tierAdmin,
		"POST /api/parse-errors/delete":             tierAdmin,
		"GET /api/users":                            tierAdmin,
		"POST /api/users":                           tierAdmin,
		"POST /api/users/{username}/role":           tierAdmin,
		"POST /api/users/{username}/full-name":      tierAdmin,
		"POST /api/users/{username}/reset-password": tierAdmin,
		"DELETE /api/users/{username}":              tierAdmin,
		"GET /api/me/search-templates":              tierLogin,
		"POST /api/me/search-templates":             tierLogin,
		"PUT /api/me/search-templates/{id}":         tierLogin,
		"DELETE /api/me/search-templates/{id}":      tierLogin,
		"GET /api/search-templates":                 tierAdmin,
		"GET /api/tokens":                           tierAdmin,
		"POST /api/tokens":                          tierAdmin,
		"DELETE /api/tokens/{id}":                   tierAdmin,
	}
}

func TestAuthMatrixCoversServerRoutes(t *testing.T) {
	want := expectedAuthMatrix()

	srv := NewServer(
		config.Config{
			ListenAddr:              ":0",
			APIAuthToken:            "test-token",
			MaxLogUploadSize:        1 << 20,
			MaxGeoUploadSize:        1 << 20,
			MaxGeoUploadRanges:      100_000,
			MaxReputationUploadSize: 1 << 20,
		},
		nil,           // ingest
		nil, nil, nil, // eventsUC, geoUC, reputationUC
		nil,           // parseErrorsUC
		nil,           // systemUC
		nil,           // systemPinger
		nil,           // parseTestUC
		nil,           // retentionUC
		nil,           // backupUC
		nil,           // authUC
		nil, nil, nil, // users, sessions, apiTokens
	)
	router, ok := srv.httpSrv.Handler.(*mux.Router)
	if !ok {
		t.Fatalf("handler type %T, want *mux.Router", srv.httpSrv.Handler)
	}

	muxRoutes := map[string]struct{}{}
	_ = router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		path, err := route.GetPathTemplate()
		if err != nil || path == "" {
			return nil
		}
		methods, err := route.GetMethods()
		if err != nil || len(methods) == 0 {
			return nil
		}
		for _, m := range methods {
			muxRoutes[strings.ToUpper(m)+" "+path] = struct{}{}
		}
		return nil
	})

	for route, tier := range want {
		switch tier {
		case tierPublic, tierLogin, tierAdmin, tierOps:
		default:
			t.Fatalf("%s: unknown tier %q", route, tier)
		}
		if _, ok := muxRoutes[route]; !ok {
			t.Errorf("auth matrix route missing in mux: %s", route)
		}
	}
	for route := range muxRoutes {
		if _, ok := want[route]; !ok {
			t.Errorf("mux route missing in auth matrix: %s", route)
		}
	}

	if want["GET /api/events"] != tierLogin {
		t.Fatal("map events must stay login-gated")
	}
	if want["GET /api/system/status"] != tierLogin {
		t.Fatal("system status pill must stay login-gated")
	}
	if want["POST /upload-logs"] != tierOps || want["POST /upload-geo"] != tierOps {
		t.Fatal("uploads require ops (admin/Bearer), not operator session alone")
	}
	if want["GET /api/system/stats"] != tierAdmin {
		t.Fatal("system stats require admin")
	}
}
