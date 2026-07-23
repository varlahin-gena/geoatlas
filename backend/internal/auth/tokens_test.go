package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTokenStoreCreateVerifyRevoke(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api_tokens.json")
	s, err := OpenOrCreateTokenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	pub, plain, err := s.Create("ci-bot", ScopeOps)
	if err != nil {
		t.Fatal(err)
	}
	if plain == "" || pub.ID == "" || pub.Scope != ScopeOps {
		t.Fatalf("pub=%#v plain empty=%v", pub, plain == "")
	}
	scope, ok := s.Verify(plain)
	if !ok || scope != ScopeOps {
		t.Fatalf("verify got %q ok=%v", scope, ok)
	}
	if _, ok := s.Verify("nm_wrong"); ok {
		t.Fatal("expected reject")
	}
	if err := s.Revoke(pub.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Verify(plain); ok {
		t.Fatal("expected revoke")
	}

	// Reload from disk.
	s2, err := LoadTokensFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Len() != 0 {
		t.Fatalf("len=%d", s2.Len())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestScopeAtLeast(t *testing.T) {
	cases := []struct {
		have, need string
		want       bool
	}{
		{ScopeAdmin, ScopeRead, true},
		{ScopeAdmin, ScopeOps, true},
		{ScopeAdmin, ScopeAdmin, true},
		{ScopeOps, ScopeRead, true},
		{ScopeOps, ScopeOps, true},
		{ScopeOps, ScopeAdmin, false},
		{ScopeRead, ScopeOps, false},
		{ScopeRead, ScopeRead, true},
		{"", ScopeRead, false},
	}
	for _, tc := range cases {
		if got := ScopeAtLeast(tc.have, tc.need); got != tc.want {
			t.Errorf("ScopeAtLeast(%q,%q)=%v want %v", tc.have, tc.need, got, tc.want)
		}
	}
}
