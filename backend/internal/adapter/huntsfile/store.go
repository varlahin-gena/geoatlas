package huntsfile

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"geoatlas/internal/fileatomic"
	"geoatlas/internal/usecase/hunts"
)

type fileData struct {
	Users map[string][]hunts.Hunt `json:"users"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Store {
	return &Store{path: strings.TrimSpace(path)}
}

func (s *Store) load() (fileData, error) {
	if s == nil || s.path == "" {
		return fileData{Users: map[string][]hunts.Hunt{}}, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileData{Users: map[string][]hunts.Hunt{}}, nil
		}
		return fileData{}, err
	}
	var out fileData
	if err := json.Unmarshal(data, &out); err != nil {
		return fileData{}, err
	}
	if out.Users == nil {
		out.Users = map[string][]hunts.Hunt{}
	}
	return out, nil
}

func (s *Store) save(data fileData) error {
	if s == nil || s.path == "" {
		return errors.New("hunts file path is empty")
	}
	return fileatomic.WriteJSON(s.path, data)
}

func (s *Store) List(username string) ([]hunts.Hunt, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, hunts.ErrInvalidName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return nil, err
	}
	out := append([]hunts.Hunt(nil), data.Users[username]...)
	return out, nil
}

func (s *Store) ListAll() ([]hunts.HuntWithAuthor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return nil, err
	}
	out := make([]hunts.HuntWithAuthor, 0)
	for user, list := range data.Users {
		for _, h := range list {
			out = append(out, hunts.HuntWithAuthor{Hunt: h, Username: user})
		}
	}
	return out, nil
}

func (s *Store) Create(username string, hunt hunts.Hunt) (hunts.Hunt, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return hunts.Hunt{}, hunts.ErrInvalidName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return hunts.Hunt{}, err
	}
	list := data.Users[username]
	if len(list) >= hunts.MaxPerUser {
		return hunts.Hunt{}, hunts.ErrLimitExceeded
	}
	total := 0
	for _, l := range data.Users {
		total += len(l)
	}
	if total >= hunts.MaxHuntsAll {
		return hunts.Hunt{}, hunts.ErrLimitExceeded
	}
	hunt.ID = newID()
	hunt.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	list = append(list, hunt)
	data.Users[username] = list
	if err := s.save(data); err != nil {
		return hunts.Hunt{}, err
	}
	return hunt, nil
}

func (s *Store) Update(username, id string, hunt hunts.Hunt) (hunts.Hunt, error) {
	username = strings.TrimSpace(username)
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return hunts.Hunt{}, err
	}
	list := data.Users[username]
	for i, h := range list {
		if h.ID != id {
			continue
		}
		hunt.ID = id
		hunt.Runs = h.Runs
		hunt.LastRunAt = h.LastRunAt
		hunt.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		list[i] = hunt
		data.Users[username] = list
		if err := s.save(data); err != nil {
			return hunts.Hunt{}, err
		}
		return hunt, nil
	}
	return hunts.Hunt{}, hunts.ErrNotFound
}

func (s *Store) Delete(username, id string) error {
	username = strings.TrimSpace(username)
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return err
	}
	list := data.Users[username]
	for i, h := range list {
		if h.ID != id {
			continue
		}
		data.Users[username] = append(list[:i], list[i+1:]...)
		return s.save(data)
	}
	return hunts.ErrNotFound
}

func (s *Store) SaveRun(username, id string, hunt hunts.Hunt) error {
	username = strings.TrimSpace(username)
	id = strings.TrimSpace(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := s.load()
	if err != nil {
		return err
	}
	list := data.Users[username]
	for i, h := range list {
		if h.ID != id {
			continue
		}
		hunt.ID = id
		list[i] = hunt
		data.Users[username] = list
		return s.save(data)
	}
	return hunts.ErrNotFound
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
