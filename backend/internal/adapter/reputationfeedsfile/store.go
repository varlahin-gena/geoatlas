package reputationfeedsfile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"network_monitor/internal/config"
)

// fileDoc — JSON на диске.
type fileDoc struct {
	Feeds []config.ReputationFeed `json:"feeds"`
}

// Store — JSON-файл с URL-фидами (том /app/data рядом с users.json).
type Store struct {
	path string
}

func New(path string) *Store {
	return &Store{path: strings.TrimSpace(path)}
}

// Load читает фиды. ok=false если файла нет.
func (s *Store) Load() (feeds []config.ReputationFeed, ok bool, err error) {
	if s == nil || s.path == "" {
		return nil, false, nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var doc fileDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, false, err
	}
	return normalizeFeeds(doc.Feeds), true, nil
}

// Save атомарно пишет фиды.
func (s *Store) Save(feeds []config.ReputationFeed) error {
	if s == nil || s.path == "" {
		return errors.New("reputation feeds file path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	doc := fileDoc{Feeds: normalizeFeeds(feeds)}
	if doc.Feeds == nil {
		doc.Feeds = []config.ReputationFeed{}
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// LoadOrSeed: файл есть → его содержимое (даже пустой список);
// файла нет → seed (или DefaultReputationFeeds) и запись.
func (s *Store) LoadOrSeed(seed []config.ReputationFeed) ([]config.ReputationFeed, error) {
	feeds, exists, err := s.Load()
	if err != nil {
		return nil, err
	}
	if exists {
		return feeds, nil
	}
	seed = normalizeFeeds(seed)
	if len(seed) == 0 {
		seed = config.DefaultReputationFeeds()
	}
	if err := s.Save(seed); err != nil {
		return seed, err
	}
	return seed, nil
}

func normalizeFeeds(in []config.ReputationFeed) []config.ReputationFeed {
	if len(in) == 0 {
		return nil
	}
	out := make([]config.ReputationFeed, 0, len(in))
	seen := map[string]struct{}{}
	for _, f := range in {
		f.Name = strings.TrimSpace(f.Name)
		f.URL = strings.TrimSpace(f.URL)
		f.Category = strings.TrimSpace(f.Category)
		f.Format = strings.ToLower(strings.TrimSpace(f.Format))
		if f.Name == "" || f.URL == "" {
			continue
		}
		if f.Category == "" {
			f.Category = "unknown"
		}
		if f.Format == "" {
			f.Format = "netset"
		}
		if _, ok := seen[f.Name]; ok {
			continue
		}
		seen[f.Name] = struct{}{}
		out = append(out, f)
	}
	return out
}
