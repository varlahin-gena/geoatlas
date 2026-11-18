package parserstore

import (
	"context"
	"testing"

	"geoatlas/internal/parser"
)

func TestStoreNilConn(t *testing.T) {
	ctx := context.Background()
	var s *Store
	if _, err := s.List(ctx); err == nil {
		t.Fatal("list nil")
	}
	s = New(nil, nil)
	if _, err := s.List(ctx); err == nil {
		t.Fatal("list nil ch")
	}
	if err := s.Upsert(ctx, parser.RuleRecord{}); err == nil {
		t.Fatal("upsert nil ch")
	}
	if err := s.Delete(ctx, "x"); err == nil {
		t.Fatal("delete nil ch")
	}
	if _, err := s.Exists(ctx, "x"); err == nil {
		t.Fatal("exists nil ch")
	}
}
