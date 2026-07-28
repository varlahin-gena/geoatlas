package reputationfeedsfile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	usecasereputation "network_monitor/internal/usecase/reputation"
)

// fileDoc — JSON на диске.
type fileDoc struct {
	Feeds []usecasereputation.Feed `json:"feeds"`
}

// Store — JSON-файл с URL-фидами (том /app/data рядом с users.json).
type Store struct {
	path string
}

func New(path string) *Store {
	return &Store{path: strings.TrimSpace(path)}
}

// Load читает фиды. ok=false если файла нет.
func (s *Store) Load() (feeds []usecasereputation.Feed, ok bool, err error) {
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
func (s *Store) Save(feeds []usecasereputation.Feed) error {
	if s == nil || s.path == "" {
		return errors.New("reputation feeds file path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	doc := fileDoc{Feeds: normalizeFeeds(feeds)}
	if doc.Feeds == nil {
		doc.Feeds = []usecasereputation.Feed{}
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
// файла нет → seed и запись.
func (s *Store) LoadOrSeed(seed []usecasereputation.Feed) ([]usecasereputation.Feed, error) {
	feeds, exists, err := s.Load()
	if err != nil {
		return nil, err
	}
	if exists {
		return feeds, nil
	}
	seed = normalizeFeeds(seed)
	if err := s.Save(seed); err != nil {
		return seed, err
	}
	return seed, nil
}

func normalizeFeeds(in []usecasereputation.Feed) []usecasereputation.Feed {
	if len(in) == 0 {
		return nil
	}
	out := make([]usecasereputation.Feed, 0, len(in))
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
