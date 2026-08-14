package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists      = errors.New("user already exists")
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidUsername = errors.New("invalid username")
	ErrInvalidPassword = errors.New("invalid password")
	ErrInvalidRole     = errors.New("invalid role")
	ErrLastAdmin       = errors.New("cannot remove or demote the last administrator")
	ErrSelfDelete      = errors.New("cannot delete your own account")
	ErrBadOldPassword  = errors.New("current password is incorrect")
	ErrInvalidFullName = errors.New("invalid full name")
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]{2,64}$`)

const (
	MinPasswordLen = 8
	MaxFullNameLen = 200
)

// User — локальная учётная запись (пароль хранится как bcrypt-хеш).
type User struct {
	Username           string `json:"username"`
	FullName           string `json:"full_name,omitempty"`
	PasswordHash       string `json:"password_hash"`
	Role               string `json:"role"`
	MustResetPassword  bool   `json:"must_reset_password"`
	GeoWizardDismissed bool   `json:"geo_wizard_dismissed,omitempty"`
	// SessionVersion — stamp в cookie; bump инвалидирует все ранее выданные сессии.
	SessionVersion int64  `json:"session_version,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
}

// UserPublic — данные без хеша пароля для API/UI.
type UserPublic struct {
	Username           string `json:"username"`
	FullName           string `json:"full_name,omitempty"`
	Role               string `json:"role"`
	MustResetPassword  bool   `json:"must_reset_password"`
	GeoWizardDismissed bool   `json:"geo_wizard_dismissed,omitempty"`
	CreatedAt          string `json:"created_at,omitempty"`
}

func (u User) Public() UserPublic {
	return UserPublic{
		Username:           u.Username,
		FullName:           u.FullName,
		Role:               u.Role,
		MustResetPassword:  u.MustResetPassword,
		GeoWizardDismissed: u.GeoWizardDismissed,
		CreatedAt:          u.CreatedAt,
	}
}

type usersFile struct {
	Users []User `json:"users"`
}

// UserStore — потокобезопасное хранилище УЗ с сохранением в JSON-файл.
type UserStore struct {
	mu      sync.RWMutex
	byLower map[string]*User
	path    string
}

func NewUserStore(users ...User) (*UserStore, error) {
	s := &UserStore{byLower: make(map[string]*User, len(users))}
	for i := range users {
		if err := s.addUnlocked(users[i]); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *UserStore) addUnlocked(u User) error {
	name := strings.TrimSpace(u.Username)
	if name == "" || !usernameRe.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidUsername, name)
	}
	if !ValidRole(u.Role) {
		return fmt.Errorf("%w: %q", ErrInvalidRole, u.Role)
	}
	if strings.TrimSpace(u.PasswordHash) == "" {
		return fmt.Errorf("user %q: empty password hash", name)
	}
	key := strings.ToLower(name)
	if _, exists := s.byLower[key]; exists {
		return fmt.Errorf("%w: %q", ErrUserExists, name)
	}
	u.Username = name
	cp := u
	s.byLower[key] = &cp
	return nil
}

// OpenOrSeed загружает users.json или создаёт из seedUsers и сохраняет.
func OpenOrSeed(path string, seedUsers []User) (*UserStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("users file path is empty")
	}
	if _, err := os.Stat(path); err == nil {
		return LoadUsersFile(path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(seedUsers) == 0 {
		return nil, fmt.Errorf("no users to seed: set AUTH_ADMIN_PASSWORD / AUTH_OPERATOR_PASSWORD or create %s", path)
	}
	s, err := NewUserStore(seedUsers...)
	if err != nil {
		return nil, err
	}
	s.path = path
	if err := s.persistUnlocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func LoadUsersFile(path string) (*UserStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f usersFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse users file: %w", err)
	}
	s, err := NewUserStore(f.Users...)
	if err != nil {
		return nil, err
	}
	if s.Len() == 0 {
		return nil, fmt.Errorf("users file %s is empty", path)
	}
	s.path = path
	return s, nil
}

func SeedUsersFromEnv(adminUser, adminPass, operatorUser, operatorPass string, adminMustReset bool) ([]User, error) {
	var users []User
	now := time.Now().UTC().Format(time.RFC3339)

	adminUser = strings.TrimSpace(adminUser)
	adminPass = strings.TrimSpace(adminPass)
	if adminUser != "" && adminPass != "" {
		hash, err := HashPassword(adminPass)
		if err != nil {
			return nil, fmt.Errorf("admin password hash: %w", err)
		}
		users = append(users, User{
			Username:          adminUser,
			FullName:          "Администратор",
			PasswordHash:      string(hash),
			Role:              RoleAdministrator,
			MustResetPassword: adminMustReset || seedPasswordWeak(adminUser, adminPass),
			CreatedAt:         now,
		})
	}

	operatorUser = strings.TrimSpace(operatorUser)
	operatorPass = strings.TrimSpace(operatorPass)
	if operatorUser != "" && operatorPass != "" {
		hash, err := HashPassword(operatorPass)
		if err != nil {
			return nil, fmt.Errorf("operator password hash: %w", err)
		}
		users = append(users, User{
			Username:          operatorUser,
			FullName:          "Оператор",
			PasswordHash:      string(hash),
			Role:              RoleOperator,
			MustResetPassword: true,
			CreatedAt:         now,
		})
	}
	return users, nil
}

func seedPasswordWeak(user, pass string) bool {
	u := strings.ToLower(strings.TrimSpace(user))
	p := strings.ToLower(strings.TrimSpace(pass))
	if p == "" {
		return false
	}
	if p == u {
		return true
	}
	switch p {
	case "admin", "operator", "password", "changeme", "123456":
		return true
	}
	return false
}

// HashPassword хеширует пароль для хранения в User.
func HashPassword(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func ValidatePassword(password string) error {
	if len(password) < MinPasswordLen {
		return fmt.Errorf("%w: minimum %d characters", ErrInvalidPassword, MinPasswordLen)
	}
	if len(password) > 128 {
		return fmt.Errorf("%w: too long", ErrInvalidPassword)
	}
	return nil
}

func NormalizeFullName(fullName string) (string, error) {
	name := strings.TrimSpace(fullName)
	// Схлопываем повторные пробелы внутри ФИО.
	name = strings.Join(strings.Fields(name), " ")
	if len([]rune(name)) > MaxFullNameLen {
		return "", fmt.Errorf("%w: maximum %d characters", ErrInvalidFullName, MaxFullNameLen)
	}
	return name, nil
}

// Authenticate проверяет логин/пароль. При успехе возвращает копию пользователя.
func (s *UserStore) Authenticate(username, password string) (*User, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	u := s.byLower[strings.ToLower(strings.TrimSpace(username))]
	if u == nil {
		const dummy = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
		_ = bcrypt.CompareHashAndPassword([]byte(dummy), []byte(password))
		return nil, false
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, false
	}
	cp := *u
	return &cp, true
}

func (s *UserStore) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byLower)
}

func (s *UserStore) Get(username string) (UserPublic, bool) {
	if s == nil {
		return UserPublic{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	u := s.byLower[strings.ToLower(strings.TrimSpace(username))]
	if u == nil {
		return UserPublic{}, false
	}
	return u.Public(), true
}

func (s *UserStore) SessionVersion(username string) (int64, bool) {
	if s == nil {
		return 0, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	u := s.byLower[strings.ToLower(strings.TrimSpace(username))]
	if u == nil {
		return 0, false
	}
	return u.SessionVersion, true
}

// BumpSessionVersion инвалидирует все cookie-сессии пользователя.
func (s *UserStore) BumpSessionVersion(username string) error {
	if s == nil {
		return ErrUserNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.byLower[strings.ToLower(strings.TrimSpace(username))]
	if u == nil {
		return ErrUserNotFound
	}
	u.SessionVersion++
	return s.persistUnlocked()
}

func (s *UserStore) MustReset(username string) bool {
	u, ok := s.Get(username)
	return ok && u.MustResetPassword
}

func (s *UserStore) List() []UserPublic {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]UserPublic, 0, len(s.byLower))
	for _, u := range s.byLower {
		out = append(out, u.Public())
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Username) < strings.ToLower(out[j].Username)
	})
	return out
}

func (s *UserStore) Create(username, password, role, fullName string, mustReset bool) (UserPublic, error) {
	if s == nil {
		return UserPublic{}, ErrUserNotFound
	}
	if err := ValidatePassword(password); err != nil {
		return UserPublic{}, err
	}
	if !ValidRole(role) {
		return UserPublic{}, ErrInvalidRole
	}
	fn, err := NormalizeFullName(fullName)
	if err != nil {
		return UserPublic{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return UserPublic{}, err
	}
	u := User{
		Username:          strings.TrimSpace(username),
		FullName:          fn,
		PasswordHash:      string(hash),
		Role:              role,
		MustResetPassword: mustReset,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.addUnlocked(u); err != nil {
		return UserPublic{}, err
	}
	if err := s.persistUnlocked(); err != nil {
		delete(s.byLower, strings.ToLower(u.Username))
		return UserPublic{}, err
	}
	return u.Public(), nil
}

func (s *UserStore) SetRole(username, role string) (UserPublic, error) {
	if s == nil {
		return UserPublic{}, ErrUserNotFound
	}
	if !ValidRole(role) {
		return UserPublic{}, ErrInvalidRole
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.byLower[strings.ToLower(strings.TrimSpace(username))]
	if u == nil {
		return UserPublic{}, ErrUserNotFound
	}
	if u.Role == RoleAdministrator && role != RoleAdministrator && s.adminCountUnlocked() <= 1 {
		return UserPublic{}, ErrLastAdmin
	}
	u.Role = role
	if err := s.persistUnlocked(); err != nil {
		return UserPublic{}, err
	}
	return u.Public(), nil
}

func (s *UserStore) SetFullName(username, fullName string) (UserPublic, error) {
	if s == nil {
		return UserPublic{}, ErrUserNotFound
	}
	fn, err := NormalizeFullName(fullName)
	if err != nil {
		return UserPublic{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.byLower[strings.ToLower(strings.TrimSpace(username))]
	if u == nil {
		return UserPublic{}, ErrUserNotFound
	}
	u.FullName = fn
	if err := s.persistUnlocked(); err != nil {
		return UserPublic{}, err
	}
	return u.Public(), nil
}

// SetGeoWizardDismissed — скрыть / снова показать first-run GeoIP wizard для УЗ.
func (s *UserStore) SetGeoWizardDismissed(username string, dismissed bool) (UserPublic, error) {
	if s == nil {
		return UserPublic{}, ErrUserNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.byLower[strings.ToLower(strings.TrimSpace(username))]
	if u == nil {
		return UserPublic{}, ErrUserNotFound
	}
	u.GeoWizardDismissed = dismissed
	if err := s.persistUnlocked(); err != nil {
		return UserPublic{}, err
	}
	return u.Public(), nil
}

func (s *UserStore) ResetPassword(username, password string, mustReset bool) (UserPublic, error) {
	if s == nil {
		return UserPublic{}, ErrUserNotFound
	}
	if err := ValidatePassword(password); err != nil {
		return UserPublic{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return UserPublic{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.byLower[strings.ToLower(strings.TrimSpace(username))]
	if u == nil {
		return UserPublic{}, ErrUserNotFound
	}
	u.PasswordHash = string(hash)
	u.MustResetPassword = mustReset
	u.SessionVersion++
	if err := s.persistUnlocked(); err != nil {
		return UserPublic{}, err
	}
	return u.Public(), nil
}

// ChangePassword — смена своего пароля; снимает must_reset_password и revoke'ит старые сессии.
func (s *UserStore) ChangePassword(username, oldPassword, newPassword string) (UserPublic, error) {
	if s == nil {
		return UserPublic{}, ErrUserNotFound
	}
	if err := ValidatePassword(newPassword); err != nil {
		return UserPublic{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.byLower[strings.ToLower(strings.TrimSpace(username))]
	if u == nil {
		return UserPublic{}, ErrUserNotFound
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPassword)) != nil {
		return UserPublic{}, ErrBadOldPassword
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return UserPublic{}, err
	}
	u.PasswordHash = string(hash)
	u.MustResetPassword = false
	u.SessionVersion++
	if err := s.persistUnlocked(); err != nil {
		return UserPublic{}, err
	}
	return u.Public(), nil
}

func (s *UserStore) Delete(username, actorUsername string) error {
	if s == nil {
		return ErrUserNotFound
	}
	key := strings.ToLower(strings.TrimSpace(username))
	actorKey := strings.ToLower(strings.TrimSpace(actorUsername))
	if key == actorKey {
		return ErrSelfDelete
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.byLower[key]
	if u == nil {
		return ErrUserNotFound
	}
	if u.Role == RoleAdministrator && s.adminCountUnlocked() <= 1 {
		return ErrLastAdmin
	}
	delete(s.byLower, key)
	if err := s.persistUnlocked(); err != nil {
		// откат
		s.byLower[key] = u
		return err
	}
	return nil
}

func (s *UserStore) adminCountUnlocked() int {
	n := 0
	for _, u := range s.byLower {
		if u.Role == RoleAdministrator {
			n++
		}
	}
	return n
}

func (s *UserStore) persistUnlocked() error {
	if s.path == "" {
		return nil // in-memory (тесты)
	}
	users := make([]User, 0, len(s.byLower))
	for _, u := range s.byLower {
		users = append(users, *u)
	}
	sort.Slice(users, func(i, j int) bool {
		return strings.ToLower(users[i].Username) < strings.ToLower(users[j].Username)
	})
	data, err := json.MarshalIndent(usersFile{Users: users}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// BuildUserStoreFromEnv — совместимость со старыми тестами (только память).
func BuildUserStoreFromEnv(adminUser, adminPass, operatorUser, operatorPass string) (*UserStore, error) {
	users, err := SeedUsersFromEnv(adminUser, adminPass, operatorUser, operatorPass, false)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("no users configured: set AUTH_ADMIN_USER/AUTH_ADMIN_PASSWORD and/or AUTH_OPERATOR_USER/AUTH_OPERATOR_PASSWORD")
	}
	// для in-memory тестов снимаем must_reset, если не файловый store
	for i := range users {
		users[i].MustResetPassword = false
	}
	return NewUserStore(users...)
}
