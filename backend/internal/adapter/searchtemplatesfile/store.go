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
	"network_monitor/internal/usecase/searchtemplates"
)

type fileData struct {
	Users map[string][]searchtemplates.Template `json:"users"`
}

// Store — потокобезопасное JSON-хранилище шаблонов по username.
type Store struct {
	mu   sync.Mutex
	path string
}

var _ searchtemplates.Store = (*Store)(nil)

func New(path string) *Store {
	return &Store{path: strings.TrimSpace(path)}
}

func (s *Store) List(username string) ([]searchtemplates.Template, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, searchtemplates.ErrInvalidName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadUnlocked()
	if err != nil {
		return nil, err
	}
	out := append([]searchtemplates.Template(nil), data.Users[username]...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out, nil
}

func (s *Store) ListAll() ([]searchtemplates.TemplateWithAuthor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadUnlocked()
	if err != nil {
		return nil, err
	}
	out := make([]searchtemplates.TemplateWithAuthor, 0)
	for user, items := range data.Users {
		for _, t := range items {
			out = append(out, searchtemplates.TemplateWithAuthor{Template: t, Username: user})
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

func (s *Store) Create(username, name, query string) (searchtemplates.Template, error) {
	username = strings.TrimSpace(username)
	name = strings.TrimSpace(name)
	query = strings.TrimSpace(query)
	if username == "" {
		return searchtemplates.Template{}, searchtemplates.ErrInvalidName
	}
	if err := searchtemplates.ValidateNameQuery(name, query); err != nil {
		return searchtemplates.Template{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadUnlocked()
	if err != nil {
		return searchtemplates.Template{}, err
	}
	list := data.Users[username]
	if len(list) >= searchtemplates.MaxPerUser {
		return searchtemplates.Template{}, searchtemplates.ErrLimitExceeded
	}
	t := searchtemplates.Template{
		ID:        newTemplateID(),
		Name:      name,
		Query:     query,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data.Users[username] = append(list, t)
	if err := s.saveUnlocked(data); err != nil {
		return searchtemplates.Template{}, err
	}
	return t, nil
}

func (s *Store) Update(username, id, name, query string) (searchtemplates.Template, error) {
	username = strings.TrimSpace(username)
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	query = strings.TrimSpace(query)
	if username == "" || id == "" {
		return searchtemplates.Template{}, searchtemplates.ErrNotFound
	}
	if err := searchtemplates.ValidateNameQuery(name, query); err != nil {
		return searchtemplates.Template{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadUnlocked()
	if err != nil {
		return searchtemplates.Template{}, err
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
			return searchtemplates.Template{}, err
		}
		return list[i], nil
	}
	return searchtemplates.Template{}, searchtemplates.ErrNotFound
}

func (s *Store) Delete(username, id string) error {
	username = strings.TrimSpace(username)
	id = strings.TrimSpace(id)
	if username == "" || id == "" {
		return searchtemplates.ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	list := data.Users[username]
	next := make([]searchtemplates.Template, 0, len(list))
	found := false
	for _, t := range list {
		if t.ID == id {
			found = true
			continue
		}
		next = append(next, t)
	}
	if !found {
		return searchtemplates.ErrNotFound
	}
	if len(next) == 0 {
		delete(data.Users, username)
	} else {
		data.Users[username] = next
	}
	return s.saveUnlocked(data)
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
		return fileData{}, searchtemplates.ErrUnavailable
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileData{Users: map[string][]searchtemplates.Template{}}, nil
		}
		return fileData{}, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fileData{Users: map[string][]searchtemplates.Template{}}, nil
	}
	var data fileData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fileData{}, fmt.Errorf("parse search templates: %w", err)
	}
	if data.Users == nil {
		data.Users = map[string][]searchtemplates.Template{}
	}
	return data, nil
}

func (s *Store) saveUnlocked(data fileData) error {
	if s == nil || s.path == "" {
		return searchtemplates.ErrUnavailable
	}
	if data.Users == nil {
		data.Users = map[string][]searchtemplates.Template{}
	}
	total := 0
	for _, list := range data.Users {
		total += len(list)
	}
	if total > searchtemplates.MaxTemplatesAll {
		return searchtemplates.ErrLimitExceeded
	}
	return fileatomic.WriteJSON(s.path, data)
}
