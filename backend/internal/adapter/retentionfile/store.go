package retentionfile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"network_monitor/internal/usecase/retention"
)

// Store — JSON-файл с TTL (том /app/data рядом с users.json).
type Store struct {
	path string
}

func New(path string) *Store {
	return &Store{path: strings.TrimSpace(path)}
}

func (s *Store) Load() (retention.Settings, error) {
	if s == nil || s.path == "" {
		return retention.Defaults(), nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return retention.Defaults(), nil
		}
		return retention.Settings{}, err
	}
	var out retention.Settings
	if err := json.Unmarshal(data, &out); err != nil {
		return retention.Settings{}, err
	}
	return out, nil
}

func (s *Store) Save(st retention.Settings) error {
	if s == nil || s.path == "" {
		return errors.New("retention file path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
