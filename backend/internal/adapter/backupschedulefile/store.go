package backupschedulefile

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"network_monitor/internal/fileatomic"
	"network_monitor/internal/usecase/backup"
)

// Store — JSON-файл расписания бэкапов (/app/data/backup_schedule.json).
type Store struct {
	path string
	seed backup.Schedule
}

func New(path string, seed backup.Schedule) *Store {
	return &Store{path: strings.TrimSpace(path), seed: seed}
}

func (s *Store) Load() (backup.Schedule, error) {
	if s == nil || s.path == "" {
		return s.seedOrDefault(), nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.seedOrDefault(), nil
		}
		return backup.Schedule{}, err
	}
	var out backup.Schedule
	if err := json.Unmarshal(data, &out); err != nil {
		return backup.Schedule{}, err
	}
	normalized, err := backup.ValidateSchedule(out)
	if err != nil {
		// битый файл — fallback на seed, но сохраняем last_run если валидны
		seed := s.seedOrDefault()
		seed.LastRunAt = strings.TrimSpace(out.LastRunAt)
		seed.LastRunDate = strings.TrimSpace(out.LastRunDate)
		return seed, nil
	}
	if normalized.LastRunAt == "" {
		normalized.LastRunAt = strings.TrimSpace(out.LastRunAt)
	}
	if normalized.LastRunDate == "" {
		normalized.LastRunDate = strings.TrimSpace(out.LastRunDate)
	}
	return normalized, nil
}

func (s *Store) Save(st backup.Schedule) error {
	if s == nil || s.path == "" {
		return errors.New("backup schedule file path is empty")
	}
	out, err := backup.ValidateSchedule(st)
	if err != nil {
		return err
	}
	out.LastRunAt = strings.TrimSpace(st.LastRunAt)
	out.LastRunDate = strings.TrimSpace(st.LastRunDate)
	out.UpdatedAt = strings.TrimSpace(st.UpdatedAt)
	return fileatomic.WriteJSON(s.path, out)
}

func (s *Store) seedOrDefault() backup.Schedule {
	if s == nil {
		return backup.DefaultsSchedule(backup.Options{Keep: 7, IncludeEdges: true, IncludeAuth: true})
	}
	out, err := backup.ValidateSchedule(s.seed)
	if err != nil {
		return backup.DefaultsSchedule(backup.Options{Keep: 7, IncludeEdges: true, IncludeAuth: true})
	}
	return out
}
