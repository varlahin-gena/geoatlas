package anomalysettingsfile

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"geoatlas/internal/fileatomic"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
)

// Store — JSON-файл настроек движка аномалий (/app/data/anomaly_settings.json).
type Store struct {
	path string
}

func New(path string) *Store {
	return &Store{path: strings.TrimSpace(path)}
}

func (s *Store) Load() (usecaseanomaly.Settings, error) {
	if s == nil || s.path == "" {
		return usecaseanomaly.Settings{}, nil
	}
	data, err := os.ReadFile(s.path)
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
	return fileatomic.WriteJSON(s.path, st)
}
