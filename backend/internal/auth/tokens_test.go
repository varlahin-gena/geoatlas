package auth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTokenStoreCreateVerifyRevoke(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api_tokens.json")
	s, err := OpenOrCreateTokenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	pub, plain, err := s.Create("ci-bot", ScopeOps, time.Time{})
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
	if _, ok := s.Verify("ga_wrong"); ok {
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

func TestTokenStoreExpiryAndRotate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "api_tokens.json")
	s, err := OpenOrCreateTokenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Create("past", ScopeRead, time.Now().UTC().Add(-time.Minute)); err == nil {
		t.Fatal("expected past expiry rejected")
	}
	exp := time.Now().UTC().Add(2 * time.Hour)
	pub, plain, err := s.Create("expiring", ScopeOps, exp)
	if err != nil {
		t.Fatal(err)
	}
	if pub.ExpiresAt == "" {
		t.Fatal("expected expires_at set")
	}
	if scope, ok := s.Verify(plain); !ok || scope != ScopeOps {
		t.Fatalf("verify before expiry: %q ok=%v", scope, ok)
	}

	rot, newPlain, err := s.Rotate(pub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rot.ID != pub.ID || newPlain == "" || newPlain == plain {
		t.Fatalf("rotate pub=%#v newPlain empty=%v same=%v", rot, newPlain == "", newPlain == plain)
	}
	if _, ok := s.Verify(plain); ok {
		t.Fatal("old secret must fail after rotate")
	}
	if scope, ok := s.Verify(newPlain); !ok || scope != ScopeOps {
		t.Fatalf("new secret verify: %q ok=%v", scope, ok)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	patched := strings.Replace(string(raw), pub.ExpiresAt, past, 1)
	if err := os.WriteFile(path, []byte(patched), 0o600); err != nil {
		t.Fatal(err)
	}
	s2, err := LoadTokensFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s2.Verify(newPlain); ok {
		t.Fatal("expired token must not verify")
	}
	if _, _, err := s2.Rotate(pub.ID); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("rotate expired: %v", err)
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
