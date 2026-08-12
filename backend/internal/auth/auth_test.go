package auth

import (
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	mgr, err := NewSessionManager("test-secret-key-32bytes-minimum!!", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, sess, err := mgr.Issue("admin", RoleAdministrator)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Username != "admin" || sess.Role != RoleAdministrator {
		t.Fatalf("sess = %+v", sess)
	}
	got, err := mgr.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "admin" || got.Role != RoleAdministrator {
		t.Fatalf("got = %+v", got)
	}
}

func TestLiveSessionRefreshesRoleAndDropsDeleted(t *testing.T) {
	hash, err := HashPassword("adminpass1")
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewUserStore(
		User{Username: "keeper", PasswordHash: string(hash), Role: RoleAdministrator},
		User{Username: "demoted", PasswordHash: string(hash), Role: RoleAdministrator},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Cookie still says administrator after demotion in store.
	sticky := Session{Username: "demoted", Role: RoleAdministrator, Expires: time.Now().Add(time.Hour).Unix()}
	if _, err := store.SetRole("demoted", RoleOperator); err != nil {
		t.Fatal(err)
	}
	live, ok := LiveSession(store, sticky)
	if !ok {
		t.Fatal("expected ok")
	}
	if live.Role != RoleOperator {
		t.Fatalf("role = %q, want operator", live.Role)
	}

	missing := Session{Username: "gone", Role: RoleOperator, Expires: sticky.Expires}
	if _, ok := LiveSession(store, missing); ok {
		t.Fatal("deleted/missing user must fail")
	}

	// nil store: keep cookie role (tests / disabled auth wiring).
	kept, ok := LiveSession(nil, sticky)
	if !ok || kept.Role != RoleAdministrator {
		t.Fatalf("nil store: %+v ok=%v", kept, ok)
	}
}

func TestUserAuthenticate(t *testing.T) {
	hash, err := HashPassword("s3cretxx")
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewUserStore(User{Username: "Admin", PasswordHash: string(hash), Role: RoleAdministrator})
	if err != nil {
		t.Fatal(err)
	}
	u, ok := store.Authenticate("admin", "s3cretxx")
	if !ok || u.Username != "Admin" {
		t.Fatalf("ok=%v user=%v", ok, u)
	}
	if _, ok := store.Authenticate("admin", "wrong"); ok {
		t.Fatal("wrong password accepted")
	}
}

func TestUserCreateResetAndMustReset(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/users.json"
	hash, err := HashPassword("adminpass1")
	if err != nil {
		t.Fatal(err)
	}
	store, err := OpenOrSeed(path, []User{{
		Username: "admin", PasswordHash: string(hash), Role: RoleAdministrator,
	}})
	if err != nil {
		t.Fatal(err)
	}
	must := true
	pub, err := store.Create("op1", "operator1", RoleOperator, "Иванов Иван Иванович", must)
	if err != nil {
		t.Fatal(err)
	}
	if !pub.MustResetPassword {
		t.Fatal("expected must_reset")
	}
	if pub.FullName != "Иванов Иван Иванович" {
		t.Fatalf("full_name = %q", pub.FullName)
	}
	if !store.MustReset("op1") {
		t.Fatal("MustReset")
	}
	if _, err := store.ResetPassword("op1", "newpass12", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ChangePassword("op1", "newpass12", "changed1"); err != nil {
		t.Fatal(err)
	}
	if store.MustReset("op1") {
		t.Fatal("must_reset should be cleared")
	}
	// re-load from disk
	store2, err := LoadUsersFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if store2.Len() != 2 {
		t.Fatalf("len=%d", store2.Len())
	}
	if _, ok := store2.Authenticate("op1", "changed1"); !ok {
		t.Fatal("reloaded auth failed")
	}
}

func TestSetGeoWizardDismissed(t *testing.T) {
	hash, err := HashPassword("adminpass1")
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewUserStore(User{
		Username: "admin", PasswordHash: string(hash), Role: RoleAdministrator,
	})
	if err != nil {
		t.Fatal(err)
	}
	pub, err := store.SetGeoWizardDismissed("admin", true)
	if err != nil {
		t.Fatal(err)
	}
	if !pub.GeoWizardDismissed {
		t.Fatal("expected dismissed")
	}
	got, ok := store.Get("admin")
	if !ok || !got.GeoWizardDismissed {
		t.Fatalf("Get dismissed=%v ok=%v", got.GeoWizardDismissed, ok)
	}
	pub, err = store.SetGeoWizardDismissed("admin", false)
	if err != nil {
		t.Fatal(err)
	}
	if pub.GeoWizardDismissed {
		t.Fatal("expected cleared")
	}
}

func TestCookieRoundTrip(t *testing.T) {
	mgr, err := NewSessionManager("cookie-secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := mgr.Issue("operator", RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := mgr.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Role != RoleOperator {
		t.Fatalf("role = %s", sess.Role)
	}
	if sess.Username != "operator" {
		t.Fatalf("username = %s", sess.Username)
	}
}
