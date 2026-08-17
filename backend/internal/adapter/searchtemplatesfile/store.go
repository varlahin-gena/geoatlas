package searchtemplatesfile

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"network_monitor/internal/fileatomic"
)

const (
	MaxNameLen      = 80
	MaxQueryLen     = 500
	MaxPerUser      = 50
	MaxTemplatesAll = 5000
)

var (
	ErrNotFound      = errors.New("template not found")
	ErrInvalidName   = errors.New("invalid template name")
	ErrInvalidQuery  = errors.New("invalid template query")
	ErrLimitExceeded = errors.New("template limit exceeded")
	ErrEmptyPath     = errors.New("search templates file path is empty")
)

// Template — именованный поисковый запрос пользователя.
type Template struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Query     string `json:"query"`
	UpdatedAt string `json:"updated_at"`
}

// TemplateWithAuthor — шаблон с автором (admin overview).
type TemplateWithAuthor struct {
	Template
	Username string `json:"username"`
}

type fileData struct {
	Users map[string][]Template `json:"users"`
}

// Store — потокобезопасное JSON-хранилище шаблонов по username.
type Store struct {
	mu   sync.Mutex
	path string
}

func New(path string) *Store {
	return &Store{path: strings.TrimSpace(path)}
}

func (s *Store) List(username string) ([]Template, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, ErrInvalidName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadUnlocked()
	if err != nil {
		return nil, err
	}
	out := append([]Template(nil), data.Users[username]...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out, nil
}

func (s *Store) ListAll() ([]TemplateWithAuthor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadUnlocked()
	if err != nil {
		return nil, err
	}
	out := make([]TemplateWithAuthor, 0)
	for user, items := range data.Users {
		for _, t := range items {
			out = append(out, TemplateWithAuthor{Template: t, Username: user})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Username != out[j].Username {
			return strings.ToLower(out[i].Username) < strings.ToLower(out[j].Username)
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out, nil
}

func (s *Store) Create(username, name, query string) (Template, error) {
	username = strings.TrimSpace(username)
	name = strings.TrimSpace(name)
	query = strings.TrimSpace(query)
	if username == "" {
		return Template{}, ErrInvalidName
	}
	if err := validateNameQuery(name, query); err != nil {
		return Template{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadUnlocked()
	if err != nil {
		return Template{}, err
	}
	list := data.Users[username]
	if len(list) >= MaxPerUser {
		return Template{}, ErrLimitExceeded
	}
	t := Template{
		ID:        newTemplateID(),
		Name:      name,
		Query:     query,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data.Users[username] = append(list, t)
	if err := s.saveUnlocked(data); err != nil {
		return Template{}, err
	}
	return t, nil
}

func (s *Store) Update(username, id, name, query string) (Template, error) {
	username = strings.TrimSpace(username)
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	query = strings.TrimSpace(query)
	if username == "" || id == "" {
		return Template{}, ErrNotFound
	}
	if err := validateNameQuery(name, query); err != nil {
		return Template{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadUnlocked()
	if err != nil {
		return Template{}, err
	}
	list := data.Users[username]
	for i := range list {
		if list[i].ID != id {
			continue
		}
		list[i].Name = name
		list[i].Query = query
		list[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		data.Users[username] = list
		if err := s.saveUnlocked(data); err != nil {
			return Template{}, err
		}
		return list[i], nil
	}
	return Template{}, ErrNotFound
}

func (s *Store) Delete(username, id string) error {
	username = strings.TrimSpace(username)
	id = strings.TrimSpace(id)
	if username == "" || id == "" {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	list := data.Users[username]
	next := make([]Template, 0, len(list))
	found := false
	for _, t := range list {
		if t.ID == id {
			found = true
			continue
		}
		next = append(next, t)
	}
	if !found {
		return ErrNotFound
	}
	if len(next) == 0 {
		delete(data.Users, username)
	} else {
		data.Users[username] = next
	}
	return s.saveUnlocked(data)
}

func validateNameQuery(name, query string) error {
	if name == "" || len([]rune(name)) > MaxNameLen {
		return ErrInvalidName
	}
	if query == "" || len([]rune(query)) > MaxQueryLen {
		return ErrInvalidQuery
	}
	return nil
}

func newTemplateID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func (s *Store) loadUnlocked() (fileData, error) {
	if s == nil || s.path == "" {
		return fileData{}, ErrEmptyPath
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileData{Users: map[string][]Template{}}, nil
		}
		return fileData{}, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fileData{Users: map[string][]Template{}}, nil
	}
	var data fileData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fileData{}, fmt.Errorf("parse search templates: %w", err)
	}
	if data.Users == nil {
		data.Users = map[string][]Template{}
	}
	return data, nil
}

func (s *Store) saveUnlocked(data fileData) error {
	if s == nil || s.path == "" {
		return ErrEmptyPath
	}
	if data.Users == nil {
		data.Users = map[string][]Template{}
	}
	total := 0
	for _, list := range data.Users {
		total += len(list)
	}
	if total > MaxTemplatesAll {
		return ErrLimitExceeded
	}
	return fileatomic.WriteJSON(s.path, data)
}
