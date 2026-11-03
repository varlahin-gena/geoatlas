package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionRegistryRegisterRevokeList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	reg, err := NewSessionRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	rec := SessionRecord{
		ID: "abc", Username: "admin", Role: RoleAdministrator, IP: "1.2.3.4",
		CreatedAt: time.Now().UTC(), LastSeenAt: time.Now().UTC(),
	}
	if err := reg.Register(rec); err != nil {
		t.Fatal(err)
	}
	if !reg.Has("abc") {
		t.Fatal("expected has")
	}
	if got := reg.List(); len(got) != 1 || got[0].ID != "abc" {
		t.Fatalf("list=%+v", got)
	}
	ok, err := reg.Revoke("abc")
	if err != nil || !ok {
		t.Fatalf("revoke %v %v", ok, err)
	}
	if reg.Has("abc") {
		t.Fatal("revoked still present")
	}

	_ = reg.Register(SessionRecord{ID: "1", Username: "a", Role: RoleOperator})
	_ = reg.Register(SessionRecord{ID: "2", Username: "a", Role: RoleOperator})
	_ = reg.Register(SessionRecord{ID: "3", Username: "b", Role: RoleOperator})
	n, err := reg.RevokeUser("a")
	if err != nil || n != 2 {
		t.Fatalf("revoke user n=%d err=%v", n, err)
	}
	if reg.Has("1") || reg.Has("2") || !reg.Has("3") {
		t.Fatalf("unexpected state %+v", reg.List())
	}
}

func TestLiveSessionCheckedRequiresRegistryWhenPresent(t *testing.T) {
	hash, err := HashPassword("adminpass1")
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewUserStore(User{Username: "alice", PasswordHash: string(hash), Role: RoleOperator})
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := NewSessionManager("test-secret-key-32bytes-minimum!!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tok, sess, err := mgr.Issue("alice", RoleOperator, 0)
	if err != nil {
		t.Fatal(err)
	}
	_ = tok
	if sess.SID == "" {
		t.Fatal("sid required")
	}
	if _, ok := LiveSessionChecked(store, nil, sess); !ok {
		t.Fatal("nil registry should allow")
	}
	reg, err := NewSessionRegistry(filepath.Join(t.TempDir(), "s.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := LiveSessionChecked(store, reg, sess); ok {
		t.Fatal("unregistered sid must fail")
	}
	if err := reg.Register(SessionRecord{ID: sess.SID, Username: "alice", Role: RoleOperator}); err != nil {
		t.Fatal(err)
	}
	if _, ok := LiveSessionChecked(store, reg, sess); !ok {
		t.Fatal("registered sid must pass")
	}
}

func TestSessionRegistryTouchExpiryAndNil(t *testing.T) {
	var nilReg *SessionRegistry
	if nilReg.Has("x") || nilReg.List() != nil {
		t.Fatal("nil registry")
	}
	nilReg.Touch("x")
	if ok, err := nilReg.Revoke("x"); ok || err != nil {
		t.Fatalf("nil revoke %v %v", ok, err)
	}
	if n, err := nilReg.RevokeUser("u"); n != 0 || err != nil {
		t.Fatalf("nil revoke user %d %v", n, err)
	}
	if err := nilReg.Register(SessionRecord{ID: "z"}); err != nil {
		t.Fatal(err)
	}

	mem, err := NewSessionRegistry("")
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.Register(SessionRecord{}); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mem.now = func() time.Time { return base }
	mem.touchMin = time.Minute
	if err := mem.Register(SessionRecord{
		ID: "live", Username: "u", Role: RoleOperator,
		ExpiresAt: base.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	mem.Touch("live")
	mem.Touch("live") // within touchMin — no-op
	mem.Touch("")
	mem.Touch("missing")
	if !mem.Has("live") {
		t.Fatal("expected live")
	}
	mem.now = func() time.Time { return base.Add(2 * time.Hour) }
	if mem.Has("live") {
		t.Fatal("expired must prune")
	}
	if got := ExpiresFromSession(Session{Expires: SessionNeverExpires}); !got.IsZero() {
		t.Fatalf("never expires -> %v", got)
	}
	if got := ExpiresFromSession(Session{Expires: base.Unix()}); !got.Equal(base) && got.Unix() != base.Unix() {
		t.Fatalf("expires map %v", got)
	}
}

func TestSessionRegistryLoadPersistedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	reg, err := NewSessionRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().UTC().Add(-time.Hour)
	future := time.Now().UTC().Add(time.Hour)
	if err := reg.Register(SessionRecord{ID: "keep", Username: "a", Role: RoleOperator, ExpiresAt: future}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(SessionRecord{ID: "gone", Username: "b", Role: RoleOperator, ExpiresAt: past}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewSessionRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.Has("keep") || reloaded.Has("gone") {
		t.Fatalf("reload list=%+v", reloaded.List())
	}
	bad := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(bad, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSessionRegistry(bad); err == nil {
		t.Fatal("expected corrupt json error")
	}
}
