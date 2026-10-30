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
func (f *fakeUsers) Create(username, _, role, fullName string, mustReset bool) (domain.UserPublic, error) {
	if f.err != nil {
		return domain.UserPublic{}, f.err
	}
	u := &domain.User{Username: username, Role: role, FullName: fullName, MustResetPassword: mustReset}
	f.user = u
	f.list = append(f.list, u.Public())
	return u.Public(), nil
}
func (f *fakeUsers) SetRole(username, role string) (domain.UserPublic, error) {
	if f.err != nil {
		return domain.UserPublic{}, f.err
	}
	if f.user == nil || f.user.Username != username {
		return domain.UserPublic{}, domain.ErrUserNotFound
	}
	f.user.Role = role
	return f.user.Public(), nil
}
func (f *fakeUsers) SetFullName(username, fullName string) (domain.UserPublic, error) {
	if f.err != nil {
		return domain.UserPublic{}, f.err
	}
	if f.user == nil || f.user.Username != username {
		return domain.UserPublic{}, domain.ErrUserNotFound
	}
	f.user.FullName = fullName
	return f.user.Public(), nil
}
func (f *fakeUsers) SetGeoWizardDismissed(username string, dismissed bool) (domain.UserPublic, error) {
	if f.err != nil {
		return domain.UserPublic{}, f.err
	}
	if f.user == nil || f.user.Username != username {
		return domain.UserPublic{}, domain.ErrUserNotFound
	}
	f.user.GeoWizardDismissed = dismissed
	return f.user.Public(), nil
}
func (f *fakeUsers) ResetPassword(username, _ string, mustReset bool) (domain.UserPublic, error) {
	if f.err != nil {
		return domain.UserPublic{}, f.err
	}
	if f.user == nil || f.user.Username != username {
		return domain.UserPublic{}, domain.ErrUserNotFound
	}
	f.user.MustResetPassword = mustReset
	f.user.SessionVersion++
	return f.user.Public(), nil
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
func (f *fakeUsers) Delete(username, _ string) error {
	if f.err != nil {
		return f.err
	}
	if f.user == nil || f.user.Username != username {
		return domain.ErrUserNotFound
	}
	f.user = nil
	return nil
}

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
	var nilSvc *Service
	if nilSvc.VerifyPassword("u", "ok") {
		t.Fatal("nil service")
	}
}

func TestSessionTTLAndMeOK(t *testing.T) {
	var nilSvc *Service
	if nilSvc.SessionTTL() != 0 {
		t.Fatal("nil ttl")
	}
	svc := New(&fakeUsers{user: &domain.User{Username: "a", Role: domain.RoleOperator}}, &fakeSessions{})
	if svc.SessionTTL() != time.Hour {
		t.Fatalf("ttl=%v", svc.SessionTTL())
	}
	pub, err := svc.Me("a")
	if err != nil || pub.Username != "a" {
		t.Fatalf("me: %+v %v", pub, err)
	}
}

func TestLoginSessionFail(t *testing.T) {
	svc := New(&fakeUsers{user: &domain.User{Username: "admin", Role: domain.RoleAdministrator}}, &fakeSessions{fail: true})
	_, err := svc.Login("admin", "ok")
	if !errors.Is(err, ErrSession) {
		t.Fatalf("want ErrSession, got %v", err)
	}
}

func TestChangePasswordErrors(t *testing.T) {
	users := &fakeUsers{err: domain.ErrUserNotFound}
	svc := New(users, &fakeSessions{})
	if _, err := svc.ChangePassword("x", "a", "b"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("propagate: %v", err)
	}
	users = &fakeUsers{user: &domain.User{Username: "admin", Role: domain.RoleAdministrator}}
	svc = New(users, &fakeSessions{fail: true})
	if _, err := svc.ChangePassword("admin", "a", "b"); !errors.Is(err, ErrSession) {
		t.Fatalf("session: %v", err)
	}
	users = &fakeUsers{user: &domain.User{Username: "admin", Role: domain.RoleAdministrator}}
	// SessionVersion lookup fails after change — force by clearing user mid-way via broken Get/SessionVersion
	broken := &fakeUsers{user: &domain.User{Username: "admin", Role: domain.RoleAdministrator}}
	svc = New(broken, &fakeSessions{})
	broken.user = nil // ChangePassword succeeds on empty? ChangePassword checks f.user — will ErrUserNotFound
	if _, err := svc.ChangePassword("admin", "a", "b"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("no user: %v", err)
	}
}

func TestListCreateRoleNameWizardResetDelete(t *testing.T) {
	users := &fakeUsers{list: nil}
	svc := New(users, &fakeSessions{})
	list, err := svc.ListUsers()
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("empty list: %+v %v", list, err)
	}
	pub, err := svc.CreateUser(CreateUserInput{
		Username: "bob", Password: "p", Role: domain.RoleOperator, FullName: "Bob", MustResetPassword: true,
	})
	if err != nil || pub.Username != "bob" {
		t.Fatalf("create: %+v %v", pub, err)
	}
	pub, err = svc.SetRole("bob", domain.RoleAdministrator)
	if err != nil || pub.Role != domain.RoleAdministrator {
		t.Fatalf("role: %+v %v", pub, err)
	}
	pub, err = svc.SetFullName("bob", "Robert")
	if err != nil || pub.FullName != "Robert" {
		t.Fatalf("name: %+v %v", pub, err)
	}
	pub, err = svc.SetGeoWizardDismissed("bob", true)
	if err != nil || !pub.GeoWizardDismissed {
		t.Fatalf("wizard: %+v %v", pub, err)
	}
	pub, err = svc.ResetPassword(ResetPasswordInput{Username: "bob", Password: "n", MustResetPassword: false})
	if err != nil || pub.MustResetPassword {
		t.Fatalf("reset: %+v %v", pub, err)
	}
	if err := svc.DeleteUser("bob", "admin"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestAdminCRUDNotConfigured(t *testing.T) {
	svc := New(nil, nil)
	if _, err := svc.ListUsers(); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("list: %v", err)
	}
	if _, err := svc.CreateUser(CreateUserInput{}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.SetRole("u", "r"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("role: %v", err)
	}
	if _, err := svc.SetFullName("u", "n"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("name: %v", err)
	}
	if _, err := svc.SetGeoWizardDismissed("u", true); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("wizard: %v", err)
	}
	if _, err := svc.ResetPassword(ResetPasswordInput{}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("reset: %v", err)
	}
	if err := svc.DeleteUser("u", "a"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("delete: %v", err)
	}
	if err := svc.LogoutAll("u"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("logout: %v", err)
	}
	if _, err := svc.Me("u"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("me: %v", err)
	}
	if _, err := svc.ChangePassword("u", "a", "b"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("chpwd: %v", err)
	}
}
