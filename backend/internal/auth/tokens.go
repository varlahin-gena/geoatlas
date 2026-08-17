package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"network_monitor/internal/fileatomic"
)

var (
	ErrTokenNotFound  = errors.New("api token not found")
	ErrInvalidTokenName = errors.New("invalid api token name")
	ErrInvalidScope   = errors.New("invalid api token scope")
	ErrTokenLimit     = errors.New("api token limit reached")
)

const (
	ScopeRead  = "read"
	ScopeOps   = "ops"
	ScopeAdmin = "admin"

	maxAPITokens = 64
	tokenBytes   = 32
)

var tokenNameRe = regexp.MustCompile(`^[a-zA-Z0-9._\- ]{2,64}$`)

var scopeRank = map[string]int{
	ScopeRead:  1,
	ScopeOps:   2,
	ScopeAdmin: 3,
}

// ValidScope reports whether scope is read|ops|admin.
func ValidScope(scope string) bool {
	_, ok := scopeRank[scope]
	return ok
}

// ScopeAtLeast — admin ⊃ ops ⊃ read.
func ScopeAtLeast(have, need string) bool {
	return scopeRank[have] >= scopeRank[need] && scopeRank[need] > 0
}

// APIToken — именованный Bearer с scope (секрет хранится как SHA-256 hex).
type APIToken struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TokenHash string `json:"token_hash"`
	Scope     string `json:"scope"`
	CreatedAt string `json:"created_at,omitempty"`
}

// APITokenPublic — без хеша (для списка).
type APITokenPublic struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Scope     string `json:"scope"`
	CreatedAt string `json:"created_at,omitempty"`
}

func (t APIToken) Public() APITokenPublic {
	return APITokenPublic{ID: t.ID, Name: t.Name, Scope: t.Scope, CreatedAt: t.CreatedAt}
}

type tokensFile struct {
	Tokens []APIToken `json:"tokens"`
}

// TokenStore — потокобезопасное хранилище API-токенов (JSON на диске).
type TokenStore struct {
	mu   sync.RWMutex
	byID map[string]*APIToken
	path string
}

// OpenOrCreateTokenStore загружает api_tokens.json или создаёт пустой файл.
func OpenOrCreateTokenStore(path string) (*TokenStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("api tokens file path is empty")
	}
	if _, err := os.Stat(path); err == nil {
		return LoadTokensFile(path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	s := &TokenStore{byID: make(map[string]*APIToken), path: path}
	if err := s.persistUnlocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func LoadTokensFile(path string) (*TokenStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f tokensFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse api tokens file: %w", err)
	}
	s := &TokenStore{byID: make(map[string]*APIToken, len(f.Tokens)), path: path}
	for _, t := range f.Tokens {
		if err := s.addUnlocked(t); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *TokenStore) addUnlocked(t APIToken) error {
	id := strings.TrimSpace(t.ID)
	name := strings.TrimSpace(t.Name)
	scope := strings.TrimSpace(t.Scope)
	hash := strings.TrimSpace(t.TokenHash)
	if id == "" {
		return fmt.Errorf("token: empty id")
	}
	if !tokenNameRe.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidTokenName, name)
	}
	if !ValidScope(scope) {
		return fmt.Errorf("%w: %q", ErrInvalidScope, scope)
	}
	if hash == "" || len(hash) != 64 {
		return fmt.Errorf("token %q: invalid hash", name)
	}
	if _, exists := s.byID[id]; exists {
		return fmt.Errorf("token id already exists: %s", id)
	}
	t.ID, t.Name, t.Scope, t.TokenHash = id, name, scope, strings.ToLower(hash)
	cp := t
	s.byID[id] = &cp
	return nil
}

func (s *TokenStore) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byID)
}

func (s *TokenStore) List() []APITokenPublic {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]APITokenPublic, 0, len(s.byID))
	for _, t := range s.byID {
		out = append(out, t.Public())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Create генерирует секрет, сохраняет хеш, возвращает plaintext один раз.
func (s *TokenStore) Create(name, scope string) (pub APITokenPublic, plaintext string, err error) {
	if s == nil {
		return APITokenPublic{}, "", fmt.Errorf("token store not configured")
	}
	name = strings.TrimSpace(name)
	scope = strings.TrimSpace(scope)
	if !tokenNameRe.MatchString(name) {
		return APITokenPublic{}, "", fmt.Errorf("%w: %q", ErrInvalidTokenName, name)
	}
	if !ValidScope(scope) {
		return APITokenPublic{}, "", fmt.Errorf("%w: %q", ErrInvalidScope, scope)
	}

	plain, err := generateAPIToken()
	if err != nil {
		return APITokenPublic{}, "", err
	}
	id, err := generateTokenID()
	if err != nil {
		return APITokenPublic{}, "", err
	}

	t := APIToken{
		ID:        id,
		Name:      name,
		TokenHash: hashAPIToken(plain),
		Scope:     scope,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.byID) >= maxAPITokens {
		return APITokenPublic{}, "", ErrTokenLimit
	}
	if err := s.addUnlocked(t); err != nil {
		return APITokenPublic{}, "", err
	}
	if err := s.persistUnlocked(); err != nil {
		delete(s.byID, id)
		return APITokenPublic{}, "", err
	}
	return t.Public(), plain, nil
}

func (s *TokenStore) Revoke(id string) error {
	if s == nil {
		return fmt.Errorf("token store not configured")
	}
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return ErrTokenNotFound
	}
	delete(s.byID, id)
	return s.persistUnlocked()
}

// Verify возвращает scope при совпадении plaintext с хешем в store.
func (s *TokenStore) Verify(plaintext string) (scope string, ok bool) {
	if s == nil {
		return "", false
	}
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return "", false
	}
	want := hashAPIToken(plaintext)
	wantb := []byte(want)

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.byID {
		hb := []byte(t.TokenHash)
		if len(hb) != len(wantb) {
			continue
		}
		if subtle.ConstantTimeCompare(hb, wantb) == 1 {
			return t.Scope, true
		}
	}
	return "", false
}

func (s *TokenStore) persistUnlocked() error {
	if s.path == "" {
		return fmt.Errorf("token store path empty")
	}
	tokens := make([]APIToken, 0, len(s.byID))
	for _, t := range s.byID {
		tokens = append(tokens, *t)
	}
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].ID < tokens[j].ID })
	data, err := json.MarshalIndent(tokensFile{Tokens: tokens}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return fileatomic.WriteFile(s.path, data, 0o600)
}

func generateAPIToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// nm_ prefix — удобно различать в логах/конфигах.
	return "nm_" + base64.RawURLEncoding.EncodeToString(b), nil
}

func generateTokenID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashAPIToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
