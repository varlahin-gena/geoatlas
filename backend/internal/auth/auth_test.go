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
	token, sess, err := mgr.Issue("admin", RoleAdministrator, 0)
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
	sticky := Session{Username: "demoted", Role: RoleAdministrator, Expires: time.Now().Add(time.Hour).Unix(), SessionVersion: 0}
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

	missing := Session{Username: "gone", Role: RoleOperator, Expires: sticky.Expires, SessionVersion: 0}
	if _, ok := LiveSession(store, missing); ok {
		t.Fatal("deleted/missing user must fail")
	}

	// nil store: keep cookie role (tests / disabled auth wiring).
	kept, ok := LiveSession(nil, sticky)
	if !ok || kept.Role != RoleAdministrator {
		t.Fatalf("nil store: %+v ok=%v", kept, ok)
	}
}

func TestLiveSessionRejectsStaleVersion(t *testing.T) {
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
	token, sess, err := mgr.Issue("alice", RoleOperator, 0)
	if err != nil {
		t.Fatal(err)
	}
	if sess.SessionVersion != 0 {
		t.Fatalf("session version = %d, want 0", sess.SessionVersion)
	}
	if err := store.BumpSessionVersion("alice"); err != nil {
		t.Fatal(err)
	}
	parsed, err := mgr.Parse(token)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := LiveSession(store, parsed); ok {
		t.Fatal("stale session version must fail")
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
	pub, err := store.Create("op1", "operator1x", RoleOperator, "Иванов Иван Иванович", must)
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
	if _, err := store.ResetPassword("op1", "newpass1234", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ChangePassword("op1", "newpass1234", "changed12a"); err != nil {
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
	if _, ok := store2.Authenticate("op1", "changed12a"); !ok {
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
	token, _, err := mgr.Issue("operator", RoleOperator, 0)
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

func TestSeedUsersFromEnvMustReset(t *testing.T) {
	users, err := SeedUsersFromEnv("admin", "correct-horse", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("len=%d, want 1 admin only", len(users))
	}
	if users[0].MustResetPassword {
		t.Fatal("chosen strong password should not force reset")
	}

	users, err = SeedUsersFromEnv("admin", "correct-horse", "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if !users[0].MustResetPassword {
		t.Fatal("AUTH_ADMIN_MUST_RESET should force reset")
	}

	users, err = SeedUsersFromEnv("admin", "admin", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !users[0].MustResetPassword {
		t.Fatal("weak seed password must force reset")
	}

	users, err = SeedUsersFromEnv("admin", "s3cretxx", "operator", "s3cretxx", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("len=%d, want admin+operator when both set", len(users))
	}
}

func TestValidatePasswordPolicy(t *testing.T) {
	if err := ValidatePassword("short1"); err == nil {
		t.Fatal("expected too short")
	}
	if err := ValidatePassword("password123"); err == nil {
		t.Fatal("expected common password reject")
	}
	if err := ValidatePassword("abcdefghij"); err == nil {
		t.Fatal("expected missing digit")
	}
	if err := ValidatePassword("1234567890"); err == nil {
		t.Fatal("expected missing letter")
	}
	if err := ValidatePassword("Correct1ab"); err != nil {
		t.Fatalf("strong password: %v", err)
	}
	if err := ValidatePasswordForUser("admin", "adminadmin1"); err == nil {
		// adminadmin1 != admin, should pass length/complexity
	} else {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidatePasswordForUser("alice", "alicealice1"); err != nil {
		// alicealice1 != alice
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidatePasswordForUser("alice", "alice"); err == nil {
		t.Fatal("expected username match reject (also too short)")
	}
	if err := ValidatePasswordForUser("LongUser12", "LongUser12"); err == nil {
		t.Fatal("expected username==password reject")
	}
}
