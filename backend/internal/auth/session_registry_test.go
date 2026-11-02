package auth

import (
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
