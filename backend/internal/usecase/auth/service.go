package auth

import (
	"errors"
	"strings"
	"time"

	domain "network_monitor/internal/auth"
)

var (
	ErrNotConfigured      = errors.New("auth not configured")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrSession            = errors.New("session error")
)

// Service — application use cases для /api/auth и /api/users.
type Service struct {
	users    UserRepository
	sessions SessionIssuer
}

func New(users UserRepository, sessions SessionIssuer) *Service {
	return &Service{users: users, sessions: sessions}
}

func (s *Service) SessionTTL() time.Duration {
	if s == nil || s.sessions == nil {
		return 0
	}
	return s.sessions.TTL()
}

type LoginResult struct {
	User  domain.User
	Token string
}

func (s *Service) Login(username, password string) (LoginResult, error) {
	if s == nil || s.users == nil || s.sessions == nil {
		return LoginResult{}, ErrNotConfigured
	}
	user, ok := s.users.Authenticate(username, password)
	if !ok || user == nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	token, _, err := s.sessions.Issue(user.Username, user.Role)
	if err != nil {
		return LoginResult{}, ErrSession
	}
	return LoginResult{User: *user, Token: token}, nil
}

func (s *Service) Me(username string) (domain.UserPublic, error) {
	if s == nil || s.users == nil {
		return domain.UserPublic{}, ErrNotConfigured
	}
	pub, found := s.users.Get(username)
	if !found {
		return domain.UserPublic{}, ErrUnauthorized
	}
	return pub, nil
}

func (s *Service) ChangePassword(username, oldPassword, newPassword string) (domain.UserPublic, error) {
	if s == nil || s.users == nil {
		return domain.UserPublic{}, ErrNotConfigured
	}
	return s.users.ChangePassword(username, oldPassword, newPassword)
}

func (s *Service) ListUsers() ([]domain.UserPublic, error) {
	if s == nil || s.users == nil {
		return nil, ErrNotConfigured
	}
	users := s.users.List()
	if users == nil {
		users = []domain.UserPublic{}
	}
	return users, nil
}

type CreateUserInput struct {
	Username          string
	FullName          string
	Password          string
	Role              string
	MustResetPassword bool
}

func (s *Service) CreateUser(in CreateUserInput) (domain.UserPublic, error) {
	if s == nil || s.users == nil {
		return domain.UserPublic{}, ErrNotConfigured
	}
	return s.users.Create(in.Username, in.Password, in.Role, in.FullName, in.MustResetPassword)
}

func (s *Service) SetRole(username, role string) (domain.UserPublic, error) {
	if s == nil || s.users == nil {
		return domain.UserPublic{}, ErrNotConfigured
	}
	return s.users.SetRole(username, role)
}

func (s *Service) SetFullName(username, fullName string) (domain.UserPublic, error) {
	if s == nil || s.users == nil {
		return domain.UserPublic{}, ErrNotConfigured
	}
	return s.users.SetFullName(username, fullName)
}

func (s *Service) SetGeoWizardDismissed(username string, dismissed bool) (domain.UserPublic, error) {
	if s == nil || s.users == nil {
		return domain.UserPublic{}, ErrNotConfigured
	}
	return s.users.SetGeoWizardDismissed(username, dismissed)
}

type ResetPasswordInput struct {
	Username          string
	Password          string
	MustResetPassword bool
}

func (s *Service) ResetPassword(in ResetPasswordInput) (domain.UserPublic, error) {
	if s == nil || s.users == nil {
		return domain.UserPublic{}, ErrNotConfigured
	}
	return s.users.ResetPassword(in.Username, in.Password, in.MustResetPassword)
}

func (s *Service) DeleteUser(username, actor string) error {
	if s == nil || s.users == nil {
		return ErrNotConfigured
	}
	return s.users.Delete(strings.TrimSpace(username), actor)
}
