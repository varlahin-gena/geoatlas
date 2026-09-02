package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONBodyRejectsUnknownFields(t *testing.T) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"username":"a","password":"b","role":"administrator"}`))
	rec := httptest.NewRecorder()
	if decodeJSONBody(rec, req, &body, defaultJSONBodyLimit) {
		t.Fatal("expected unknown field rejection")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestDecodeJSONBodyRejectsTrailingJSON(t *testing.T) {
	var body struct {
		Username string `json:"username"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"username":"a"}{}`))
	rec := httptest.NewRecorder()
	if decodeJSONBody(rec, req, &body, defaultJSONBodyLimit) {
		t.Fatal("expected trailing json rejection")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestDecodeJSONBodyAcceptsValidPayload(t *testing.T) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"username":"admin","password":"secret"}`))
	rec := httptest.NewRecorder()
	if !decodeJSONBody(rec, req, &body, defaultJSONBodyLimit) {
		t.Fatalf("unexpected failure: %s", rec.Body.String())
	}
	if body.Username != "admin" || body.Password != "secret" {
		t.Fatalf("body = %+v", body)
	}
}
