package auth

import (
	"time"

	domain "network_monitor/internal/auth"
)

// UserRepository — хранилище учётных записей (реализация: *auth.UserStore).
type UserRepository interface {
	Authenticate(username, password string) (*domain.User, bool)
	Get(username string) (domain.UserPublic, bool)
	List() []domain.UserPublic
	Create(username, password, role, fullName string, mustReset bool) (domain.UserPublic, error)
	SetRole(username, role string) (domain.UserPublic, error)
	SetFullName(username, fullName string) (domain.UserPublic, error)
	SetGeoWizardDismissed(username string, dismissed bool) (domain.UserPublic, error)
	ResetPassword(username, password string, mustReset bool) (domain.UserPublic, error)
	ChangePassword(username, oldPassword, newPassword string) (domain.UserPublic, error)
	Delete(username, actorUsername string) error
}

// SessionIssuer — выдача сессионных токенов (реализация: *auth.SessionManager).
// Cookie/CSRF остаются в HTTP-слое.
type SessionIssuer interface {
	Issue(username, role string) (string, domain.Session, error)
	TTL() time.Duration
}
