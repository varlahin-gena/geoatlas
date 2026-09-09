package anomalysettingsfile

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"

	"geoatlas/internal/fileatomic"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
)

// Store — JSON-файл настроек движка аномалий (/app/data/anomaly_settings.json).
// mu сериализует Load/Save при параллельных PUT /api/anomalies/settings.
type Store struct {
	mu   sync.RWMutex
	path string
}

func New(path string) *Store {
	return &Store{path: strings.TrimSpace(path)}
}

func (s *Store) Load() (usecaseanomaly.Settings, error) {
	if s == nil || s.path == "" {
		return usecaseanomaly.Settings{}, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := fileatomic.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return usecaseanomaly.Settings{}, nil
		}
		return usecaseanomaly.Settings{}, err
	}
	var out usecaseanomaly.Settings
	if err := json.Unmarshal(data, &out); err != nil {
		return usecaseanomaly.Settings{}, err
	}
	return out, nil
}

func (s *Store) Save(st usecaseanomaly.Settings) error {
	if s == nil || s.path == "" {
		return errors.New("anomaly settings file path is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return fileatomic.WriteJSON(s.path, st)
}
