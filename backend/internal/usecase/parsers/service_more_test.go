package parsers

import (
	"context"
	"errors"
	"testing"

	"geoatlas/internal/apperr"
	"geoatlas/internal/parser"
)

func TestReorderDryRunAndGuards(t *testing.T) {
	ctx := context.Background()
	store := newMem()
	rl := &reloadCount{}
	svc := New(store, rl)
	if err := svc.EnsureSeed(ctx); err != nil {
		t.Fatal(err)
	}

	if err := svc.Reorder(ctx, nil); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("empty order: %v", err)
	}
	if err := svc.Reorder(ctx, []OrderEntry{
		{ID: "usergate", Priority: 50},
		{ID: "cisco-asa", Priority: 1},
	}); err != nil {
		t.Fatal(err)
	}
	ug, err := svc.Get(ctx, "usergate")
	if err != nil || ug.Priority != 50 {
		t.Fatalf("priority=%d err=%v", ug.Priority, err)
	}

	line := `CEF:0|Usergate|UTM|6|traffic|firewall|1|rt=1652344423822 deviceExternalId=utm src=10.10.10.10 spt=1 dst=1.2.3.4 dpt=443 act=accept proto=TCP in=1 out=2`
	res, err := svc.DryRun(ctx, DryRunInput{Text: line, RuleID: "usergate"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Parsed < 1 {
		t.Fatalf("parsed=%d", res.Parsed)
	}

	spec := parser.RuleSpec{
		Kind:   parser.RuleKindKV,
		Match:  parser.RuleMatch{RequiredKeys: []string{"src", "dst"}},
		Fields: parser.FieldMap{Src: "src", Dst: "dst", Act: "act"},
	}
	res, err = svc.DryRun(ctx, DryRunInput{
		Text: "src=10.0.0.1 dst=10.0.0.2 act=allow",
		Spec: &spec, Vendor: "x", Name: "y",
	}, nil)
	if err != nil || res.Parsed != 1 {
		t.Fatalf("spec dry-run parsed=%d err=%v", res.Parsed, err)
	}

	reg, err := parser.BuildRegistry(parser.DefaultSeedRules())
	if err != nil {
		t.Fatal(err)
	}
	res, err = svc.DryRun(ctx, DryRunInput{Text: "src=192.0.2.1 dst=198.51.100.1 act=deny"}, reg.ParseVerbose)
	if err != nil {
		t.Fatal(err)
	}
	if res.Received != 1 {
		t.Fatalf("received=%d", res.Received)
	}

	if _, err := svc.DryRun(ctx, DryRunInput{}, nil); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("empty text: %v", err)
	}
	if _, err := svc.Update(ctx, "cisco-ftd", UpdateInput{}); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("builtin update: %v", err)
	}
	if err := svc.Delete(ctx, "cisco-ftd"); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("builtin delete: %v", err)
	}

	name := "UG2"
	vendor := "usergate"
	pri := 110
	if ug.Spec == nil {
		t.Fatal("seed spec nil")
	}
	spec2 := *ug.Spec
	_, err = svc.Update(ctx, "usergate", UpdateInput{Name: &name, Vendor: &vendor, Priority: &pri, Spec: &spec2})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Create(ctx, CreateInput{ID: "!!!", Spec: spec})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("bad id: %v", err)
	}

	_, err = svc.Create(ctx, CreateInput{
		ID: "usergate",
		Spec: parser.RuleSpec{
			Kind: parser.RuleKindCEF, Match: parser.RuleMatch{CEFVendor: "X"},
			Fields: parser.FieldMap{Src: "src", Dst: "dst"},
		},
	})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("dup: %v", err)
	}
}

func TestNilServiceGuards(t *testing.T) {
	var s *Service
	if err := s.EnsureSeed(context.Background()); err != nil {
		t.Fatal(err)
	}
	s = New(nil, nil)
	if err := s.EnsureSeed(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.List(context.Background()); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("list: %v", err)
	}
}
