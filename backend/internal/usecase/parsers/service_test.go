package parsers

import (
	"context"
	"errors"
	"sync"
	"testing"

	"geoatlas/internal/apperr"
	"geoatlas/internal/parser"
)

type memStore struct {
	mu   sync.Mutex
	byID map[string]parser.RuleRecord
}

func newMem() *memStore {
	return &memStore{byID: map[string]parser.RuleRecord{}}
}

func (m *memStore) List(context.Context) ([]parser.RuleRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]parser.RuleRecord, 0, len(m.byID))
	for _, r := range m.byID {
		out = append(out, r)
	}
	return out, nil
}

func (m *memStore) Upsert(_ context.Context, rec parser.RuleRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[rec.ID] = rec
	return nil
}

func (m *memStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.byID, id)
	return nil
}

func (m *memStore) Exists(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.byID[id]
	return ok, nil
}

type reloadCount struct{ n int }

func (r *reloadCount) Reload(context.Context) error { r.n++; return nil }

func TestEnsureSeedAndCRUD(t *testing.T) {
	ctx := context.Background()
	store := newMem()
	rl := &reloadCount{}
	svc := New(store, rl)

	if err := svc.EnsureSeed(ctx); err != nil {
		t.Fatal(err)
	}
	if rl.n != 1 {
		t.Fatalf("reload after seed: %d", rl.n)
	}
	items, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// 3 seeds + 2 builtins
	if len(items) != 5 {
		t.Fatalf("len=%d want 5", len(items))
	}

	// second seed is no-op
	if err := svc.EnsureSeed(ctx); err != nil {
		t.Fatal(err)
	}
	if rl.n != 1 {
		t.Fatalf("reload should not bump: %d", rl.n)
	}

	rec, err := svc.Create(ctx, CreateInput{
		ID: "acme", Name: "Acme CEF", Vendor: "acme", Priority: 150,
		Spec: parser.RuleSpec{
			Kind:   parser.RuleKindCEF,
			Match:  parser.RuleMatch{CEFVendor: "Acme"},
			Fields: parser.FieldMap{Src: "src", Dst: "dst"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID != "acme" {
		t.Fatal(rec.ID)
	}

	_, err = svc.Create(ctx, CreateInput{ID: "cisco-asa", Spec: rec.Spec})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}

	off := false
	_, err = svc.Update(ctx, "acme", UpdateInput{Enabled: &off})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, "acme")
	if err != nil || got.Enabled {
		t.Fatalf("enabled=%v err=%v", got.Enabled, err)
	}

	if err := svc.Delete(ctx, "acme"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, "acme"); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}
