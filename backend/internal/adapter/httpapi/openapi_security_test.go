package httpapi

import (
	"fmt"
	"strings"
	"testing"
)

// mutationPaths lists POST/PUT/PATCH endpoints whose JSON requestBody must block
// mass assignment (additionalProperties: false on object schemas).
var mutationPaths = []struct {
	method string
	path   string
}{
	{"post", "/api/auth/login"},
	{"post", "/api/auth/geo-wizard-dismiss"},
	{"post", "/api/tokens"},
	{"post", "/api/anomalies/{fingerprint}/assign"},
	{"put", "/api/anomalies/settings"},
	{"post", "/api/geo-ranges"},
	{"put", "/api/geo-ranges"},
	{"post", "/api/enterprise-nets"},
	{"post", "/api/reputation/feeds"},
	{"put", "/api/system/retention"},
	{"put", "/api/system/tls"},
	{"put", "/api/system/backup-schedule"},
	{"post", "/api/parse-errors/delete"},
	{"post", "/api/me/hunts"},
	{"put", "/api/me/hunts/{id}"},
}

func requestBodySchema(t *testing.T, doc openAPIDoc, method, path string) map[string]any {
	t.Helper()
	ops, ok := doc.Paths[path]
	if !ok {
		t.Fatalf("path %q missing from openapi", path)
	}
	op, ok := ops[strings.ToLower(method)].(map[string]any)
	if !ok {
		t.Fatalf("%s %s missing from openapi", method, path)
	}
	rb, ok := op["requestBody"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s: requestBody missing", method, path)
	}
	content, ok := rb["content"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s: requestBody.content missing", method, path)
	}
	appJSON, ok := content["application/json"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s: application/json request body missing", method, path)
	}
	schema, ok := appJSON["schema"].(map[string]any)
	if !ok {
		t.Fatalf("%s %s: schema missing", method, path)
	}
	return schema
}

func resolveSchema(t *testing.T, doc openAPIDoc, schema map[string]any) map[string]any {
	t.Helper()
	if ref, ok := schema["$ref"].(string); ok {
		name := strings.TrimPrefix(ref, "#/components/schemas/")
		resolved, ok := doc.Components.Schemas[name].(map[string]any)
		if !ok {
			t.Fatalf("schema ref %q not found", ref)
		}
		return resolved
	}
	return schema
}

func schemaBlocksMassAssignment(t *testing.T, doc openAPIDoc, schema map[string]any) {
	t.Helper()
	schema = resolveSchema(t, doc, schema)
	if typ, _ := schema["type"].(string); typ != "object" {
		return
	}
	if ap, ok := schema["additionalProperties"].(bool); ok && !ap {
		return
	}
	if ap, ok := schema["additionalProperties"]; ok && ap == false {
		return
	}
	t.Fatalf("object schema must set additionalProperties: false (got %v)", schema["additionalProperties"])
}

func TestOpenAPIMutationRequestBodiesBlockMassAssignment(t *testing.T) {
	doc := loadOpenAPIDoc(t)
	for _, tc := range mutationPaths {
		tc := tc
		t.Run(fmt.Sprintf("%s %s", strings.ToUpper(tc.method), tc.path), func(t *testing.T) {
			schema := requestBodySchema(t, doc, tc.method, tc.path)
			schemaBlocksMassAssignment(t, doc, schema)
		})
	}
}

func TestOpenAPIAuthUserBlocksExtraFields(t *testing.T) {
	doc := loadOpenAPIDoc(t)
	schema, ok := doc.Components.Schemas["AuthUser"].(map[string]any)
	if !ok {
		t.Fatal("AuthUser schema missing")
	}
	if ap, ok := schema["additionalProperties"].(bool); !ok || ap {
		t.Fatalf("AuthUser additionalProperties = %v, want false", schema["additionalProperties"])
	}
}

func TestOpenAPIRetentionSettingsBlocksExtraFields(t *testing.T) {
	doc := loadOpenAPIDoc(t)
	schemaBlocksMassAssignment(t, doc, map[string]any{
		"$ref": "#/components/schemas/RetentionSettings",
	})
}
