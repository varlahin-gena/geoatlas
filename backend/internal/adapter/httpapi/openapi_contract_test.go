package httpapi

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"network_monitor/internal/config"
)

type openAPIDoc struct {
	Paths map[string]map[string]any `yaml:"paths"`
}

func TestOpenAPIPathsMatchMux(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	openapiPath := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "openapi.yaml"))
	raw, err := os.ReadFile(openapiPath)
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	var doc openAPIDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse openapi: %v", err)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("openapi paths empty")
	}

	srv := NewServer(Params{
		Cfg: config.Config{
			ListenAddr:              ":0",
			APIAuthToken:            "test-token",
			MaxLogUploadSize:        1 << 20,
			MaxGeoUploadSize:        1 << 20,
			MaxGeoUploadRanges:      100_000,
			MaxReputationUploadSize: 1 << 20,
			QueryTimeout:            0,
		},
	})

	muxRoutes := map[string]map[string]struct{}{} // path -> methods
	for _, route := range srv.Routes() {
		if route.Path == "" || route.Method == "" {
			continue
		}
		if muxRoutes[route.Path] == nil {
			muxRoutes[route.Path] = map[string]struct{}{}
		}
		muxRoutes[route.Path][strings.ToUpper(route.Method)] = struct{}{}
	}

	httpMethods := map[string]struct{}{
		"get": {}, "post": {}, "put": {}, "patch": {}, "delete": {}, "head": {}, "options": {},
	}

	for path, ops := range doc.Paths {
		for method := range ops {
			method = strings.ToLower(method)
			if _, isHTTP := httpMethods[method]; !isHTTP {
				continue // parameters, summary, etc.
			}
			want := strings.ToUpper(method)
			got, exists := muxRoutes[path]
			if !exists {
				t.Errorf("openapi %s %s: path missing in mux", want, path)
				continue
			}
			if _, ok := got[want]; !ok {
				t.Errorf("openapi %s %s: method missing in mux (have %v)", want, path, keysOf(got))
			}
		}
	}

	for path, methods := range muxRoutes {
		specOps, exists := doc.Paths[path]
		if !exists {
			t.Errorf("mux path %s not documented in openapi.yaml", path)
			continue
		}
		for m := range methods {
			if _, ok := specOps[strings.ToLower(m)]; !ok {
				t.Errorf("mux %s %s not documented in openapi.yaml", m, path)
			}
		}
	}
}

func keysOf(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
