package httpapi

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"geoatlas/internal/auth"
	"geoatlas/internal/config"
)

type openAPIDoc struct {
	Paths      map[string]map[string]any `yaml:"paths"`
	Components struct {
		Schemas   map[string]any `yaml:"schemas"`
		Responses map[string]any `yaml:"responses"`
	} `yaml:"components"`
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

func loadOpenAPIDoc(t *testing.T) openAPIDoc {
	t.Helper()
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
	return doc
}

func schemaProps(t *testing.T, doc openAPIDoc, name string) map[string]any {
	t.Helper()
	schema, ok := doc.Components.Schemas[name].(map[string]any)
	if !ok {
		t.Fatalf("schema %q missing", name)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema %q: no properties", name)
	}
	return props
}

func schemaRequired(t *testing.T, doc openAPIDoc, name string) []string {
	t.Helper()
	schema, ok := doc.Components.Schemas[name].(map[string]any)
	if !ok {
		t.Fatalf("schema %q missing", name)
	}
	raw, ok := schema["required"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func pathResponses(t *testing.T, doc openAPIDoc, path, method string) map[string]any {
	t.Helper()
	ops, ok := doc.Paths[path]
	if !ok {
		t.Fatalf("path %q missing", path)
	}
	op, ok := ops[strings.ToLower(method)].(map[string]any)
	if !ok {
		t.Fatalf("%s %s missing", method, path)
	}
	resp, ok := op["responses"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s: no responses", method, path)
	}
	return resp
}

func TestOpenAPIAuthUserSchemaMatchesHandler(t *testing.T) {
	doc := loadOpenAPIDoc(t)
	required := schemaRequired(t, doc, "AuthUser")
	wantRequired := map[string]struct{}{
		"username": {}, "role": {}, "reputationEnabled": {},
	}
	for _, f := range required {
		if _, ok := wantRequired[f]; !ok {
			t.Errorf("AuthUser required %q not in handler contract", f)
		}
		delete(wantRequired, f)
	}
	for f := range wantRequired {
		t.Errorf("handler AuthUser missing required openapi field %q", f)
	}

	props := schemaProps(t, doc, "AuthUser")
	handlerFields := map[string]struct{}{
		"username": {}, "full_name": {}, "role": {}, "must_reset_password": {},
		"geo_wizard_dismissed": {}, "authDisabled": {}, "reputationEnabled": {},
	}
	for f := range props {
		if _, ok := handlerFields[f]; !ok {
			t.Errorf("openapi AuthUser.%q not emitted by authUserResponse", f)
		}
	}
}

func TestOpenAPIEventsResponseSchemaMatchesHandler(t *testing.T) {
	doc := loadOpenAPIDoc(t)
	props := schemaProps(t, doc, "EventsResponse")
	handlerTop := map[string]struct{}{
		"group_by": {}, "filter": {}, "country": {}, "q": {}, "period": {},
		"data_source": {}, "backup_attached": {}, "lines": {}, "points": {},
		"reputation_facets": {}, "stats": {},
		"days": {}, "hours": {}, "minutes": {}, "from": {}, "to": {},
	}
	for f := range props {
		if _, ok := handlerTop[f]; !ok {
			t.Errorf("openapi EventsResponse.%q not in GET /api/events payload", f)
		}
	}
	statsProps, ok := props["stats"].(map[string]any)["properties"].(map[string]any)
	if !ok {
		t.Fatal("EventsResponse.stats.properties missing")
	}
	for _, f := range []string{"raw_pairs", "edges", "nodes", "skipped_no_geo", "source"} {
		if _, ok := statsProps[f]; !ok {
			t.Errorf("EventsResponse.stats.%q missing in openapi", f)
		}
	}
}

func TestOpenAPIAuthEventsStatusCodes(t *testing.T) {
	doc := loadOpenAPIDoc(t)
	login := pathResponses(t, doc, "/api/auth/login", "post")
	for _, code := range []string{"200", "401"} {
		if _, ok := login[code]; !ok {
			t.Errorf("POST /api/auth/login missing %s response", code)
		}
	}
	events := pathResponses(t, doc, "/api/events", "get")
	for _, code := range []string{"200", "400"} {
		if _, ok := events[code]; !ok {
			t.Errorf("GET /api/events missing %s response", code)
		}
	}
	login200, ok := login["200"].(map[string]any)
	if !ok {
		t.Fatal("login 200 missing")
	}
	content, ok := login200["content"].(map[string]any)
	if !ok {
		t.Fatal("login 200 content missing")
	}
	appJSON, ok := content["application/json"].(map[string]any)
	if !ok {
		t.Fatal("login 200 application/json missing")
	}
	ref, ok := appJSON["schema"].(map[string]any)["$ref"].(string)
	if !ok || !strings.HasSuffix(ref, "AuthUser") {
		t.Fatalf("login 200 schema ref = %v", appJSON["schema"])
	}
	events200, ok := events["200"].(map[string]any)
	if !ok {
		t.Fatal("events 200 missing")
	}
	content, ok = events200["content"].(map[string]any)
	if !ok {
		t.Fatal("events 200 content missing")
	}
	appJSON, ok = content["application/json"].(map[string]any)
	if !ok {
		t.Fatal("events 200 application/json missing")
	}
	ref, ok = appJSON["schema"].(map[string]any)["$ref"].(string)
	if !ok || !strings.HasSuffix(ref, "EventsResponse") {
		t.Fatalf("events 200 schema ref = %v", appJSON["schema"])
	}
}

func roleEnumFromSchema(t *testing.T, doc openAPIDoc, schemaName string) []string {
	t.Helper()
	props := schemaProps(t, doc, schemaName)
	roleProp, ok := props["role"].(map[string]any)
	if !ok {
		t.Fatalf("schema %q: role property missing", schemaName)
	}
	raw, ok := roleProp["enum"].([]any)
	if !ok || len(raw) == 0 {
		t.Fatalf("schema %q: role.enum missing", schemaName)
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok || s == "" {
			t.Fatalf("schema %q: invalid role enum entry %v", schemaName, v)
		}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func TestOpenAPIRoleEnumsMatchAuth(t *testing.T) {
	doc := loadOpenAPIDoc(t)
	want := append([]string(nil), auth.AllRoles()...)
	sort.Strings(want)

	for _, schema := range []string{"AuthUser", "AuthUserPublic"} {
		got := roleEnumFromSchema(t, doc, schema)
		if len(got) != len(want) {
			t.Fatalf("%s role enum = %v, want %v", schema, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s role enum = %v, want %v", schema, got, want)
			}
		}
	}
}
