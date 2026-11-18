package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"geoatlas/internal/parser"
	"geoatlas/internal/usecase/parsers"
)

func callWithPathID(h http.HandlerFunc, method, path, id, body string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

type memParserStore struct {
	byID map[string]parser.RuleRecord
}

func (m *memParserStore) List(context.Context) ([]parser.RuleRecord, error) {
	out := make([]parser.RuleRecord, 0, len(m.byID))
	for _, r := range m.byID {
		out = append(out, r)
	}
	return out, nil
}
func (m *memParserStore) Upsert(_ context.Context, rec parser.RuleRecord) error {
	if m.byID == nil {
		m.byID = map[string]parser.RuleRecord{}
	}
	m.byID[rec.ID] = rec
	return nil
}
func (m *memParserStore) Delete(_ context.Context, id string) error {
	delete(m.byID, id)
	return nil
}
func (m *memParserStore) Exists(_ context.Context, id string) (bool, error) {
	_, ok := m.byID[id]
	return ok, nil
}

type nopReload struct{}

func (nopReload) Reload(context.Context) error { return nil }

func TestParsersHandlerCRUD(t *testing.T) {
	store := &memParserStore{byID: map[string]parser.RuleRecord{}}
	uc := parsers.New(store, nopReload{})
	_ = uc.EnsureSeed(context.Background())
	reg, _ := parser.BuildRegistry(parser.DefaultSeedRules())
	h := &ParseHandler{ParseDeps: &ParseDeps{
		queryTimeout: time.Second,
		parsersUC:    uc,
		liveParse:    reg,
	}}

	rec := call(h.ListParsers, http.MethodGet, "/api/parsers", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list %d %s", rec.Code, rec.Body.String())
	}
	rec = callWithPathID(h.GetParser, http.MethodGet, "/api/parsers/usergate", "usergate", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d", rec.Code)
	}
	rec = call(h.CreateParser, http.MethodPost, "/api/parsers", `{
		"id":"acme","name":"Acme","vendor":"acme","priority":150,
		"spec":{"kind":"kv","match":{"required_keys":["src","dst"]},"fields":{"src":"src","dst":"dst"}}
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	rec = callWithPathID(h.UpdateParser, http.MethodPut, "/api/parsers/acme", "acme", `{"enabled":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.ReorderParsers, http.MethodPut, "/api/parsers/order", `{"order":[{"id":"acme","priority":40}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("reorder %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h.DryRunParser, http.MethodPost, "/api/parsers/dry-run", `{
		"text":"src=10.0.0.1 dst=10.0.0.2 act=allow",
		"rule_id":"generic"
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run %d %s", rec.Code, rec.Body.String())
	}
	rec = callWithPathID(h.DeleteParser, http.MethodDelete, "/api/parsers/acme", "acme", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("delete %d %s", rec.Code, rec.Body.String())
	}

	rec = call(h.CreateParser, http.MethodPost, "/api/parsers", `{not-json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad json %d", rec.Code)
	}
}

func TestParsersHandlerUnavailable(t *testing.T) {
	h := &ParseHandler{ParseDeps: &ParseDeps{queryTimeout: time.Second}}
	for _, fn := range []http.HandlerFunc{
		h.ListParsers, h.GetParser, h.CreateParser, h.UpdateParser,
		h.DeleteParser, h.ReorderParsers, h.DryRunParser,
	} {
		rec := call(fn, http.MethodGet, "/api/parsers", "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("code=%d", rec.Code)
		}
	}
}
