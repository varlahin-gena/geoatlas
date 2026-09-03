package auth

import (
	"errors"
	"testing"
	"time"

	domain "geoatlas/internal/auth"
)

type fakeUsers struct {
	user *domain.User
	list []domain.UserPublic
	err  error
}

func (f *fakeUsers) Authenticate(username, password string) (*domain.User, bool) {
	if f.user == nil || f.user.Username != username || password != "ok" {
		return nil, false
	}
	cp := *f.user
	return &cp, true
}
func (f *fakeUsers) Get(username string) (domain.UserPublic, bool) {
	if f.user == nil || f.user.Username != username {
		return domain.UserPublic{}, false
	}
	return f.user.Public(), true
}
func (f *fakeUsers) SessionVersion(username string) (int64, bool) {
	if f.user == nil || f.user.Username != username {
		return 0, false
	}
	return f.user.SessionVersion, true
}
func (f *fakeUsers) BumpSessionVersion(username string) error {
	if f.user == nil || f.user.Username != username {
		return domain.ErrUserNotFound
	}
	f.user.SessionVersion++
	return nil
}
func (f *fakeUsers) List() []domain.UserPublic { return f.list }
func (f *fakeUsers) Create(string, string, string, string, bool) (domain.UserPublic, error) {
	return domain.UserPublic{}, f.err
}
func (f *fakeUsers) SetRole(string, string) (domain.UserPublic, error) {
	return domain.UserPublic{}, f.err
}
func (f *fakeUsers) SetFullName(string, string) (domain.UserPublic, error) {
	return domain.UserPublic{}, f.err
}
func (f *fakeUsers) SetGeoWizardDismissed(string, bool) (domain.UserPublic, error) {
	if f.err != nil {
		return domain.UserPublic{}, f.err
	}
	if f.user == nil {
		return domain.UserPublic{}, domain.ErrUserNotFound
	}
	f.user.GeoWizardDismissed = true
	return f.user.Public(), nil
}
func (f *fakeUsers) ResetPassword(string, string, bool) (domain.UserPublic, error) {
	return domain.UserPublic{}, f.err
}
func (f *fakeUsers) ChangePassword(string, string, string) (domain.UserPublic, error) {
	if f.err != nil {
		return domain.UserPublic{}, f.err
	}
	if f.user == nil {
		return domain.UserPublic{}, domain.ErrUserNotFound
	}
	// bump session version on success like real store
	f.user.SessionVersion++
	return f.user.Public(), nil
}
func (f *fakeUsers) Delete(string, string) error { return f.err }

type fakeSessions struct {
	fail bool
}

func (f *fakeSessions) Issue(username, role string, sessionVersion int64) (string, domain.Session, error) {
	if f.fail {
		return "", domain.Session{}, errors.New("boom")
	}
	return "tok", domain.Session{Username: username, Role: role, SessionVersion: sessionVersion}, nil
}
func (f *fakeSessions) TTL() time.Duration { return time.Hour }

func TestLoginSuccess(t *testing.T) {
	svc := New(&fakeUsers{user: &domain.User{Username: "admin", Role: domain.RoleAdministrator}}, &fakeSessions{})
	out, err := svc.Login("admin", "ok")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if out.Token != "tok" || out.User.Username != "admin" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestLoginInvalid(t *testing.T) {
	svc := New(&fakeUsers{user: &domain.User{Username: "admin", Role: domain.RoleAdministrator}}, &fakeSessions{})
	_, err := svc.Login("admin", "bad")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginNotConfigured(t *testing.T) {
	_, err := New(nil, nil).Login("a", "b")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestMeUnauthorized(t *testing.T) {
	svc := New(&fakeUsers{}, &fakeSessions{})
	_, err := svc.Me("ghost")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestChangePasswordReissuesToken(t *testing.T) {
	users := &fakeUsers{user: &domain.User{Username: "admin", Role: domain.RoleAdministrator, SessionVersion: 0}}
	svc := New(users, &fakeSessions{})
	out, err := svc.ChangePassword("admin", "old", "new")
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if out.Token != "tok" {
		t.Fatalf("token = %q, want tok", out.Token)
	}
	if users.user.SessionVersion != 1 {
		t.Fatalf("session version = %d, want 1", users.user.SessionVersion)
	}
}

func TestLogoutAll(t *testing.T) {
	users := &fakeUsers{user: &domain.User{Username: "admin", Role: domain.RoleAdministrator, SessionVersion: 2}}
	svc := New(users, &fakeSessions{})
	if err := svc.LogoutAll("admin"); err != nil {
		t.Fatalf("LogoutAll: %v", err)
	}
	if users.user.SessionVersion != 3 {
		t.Fatalf("session version = %d, want 3", users.user.SessionVersion)
	}
}

func TestVerifyPassword(t *testing.T) {
	users := &fakeUsers{user: &domain.User{Username: "u1", Role: domain.RoleAdministrator}}
	svc := New(users, nil)
	if !svc.VerifyPassword("u1", "ok") {
		t.Fatal("expected valid password")
	}
	if svc.VerifyPassword("u1", "bad") {
		t.Fatal("expected invalid password")
	}
}
