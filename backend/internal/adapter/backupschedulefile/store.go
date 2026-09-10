package backupschedulefile

import (
	"strings"

	"geoatlas/internal/jsonfile"
	"geoatlas/internal/usecase/backup"
)

// Store — JSON-файл расписания бэкапов (/app/data/backup_schedule.json).
type Store = jsonfile.Store[backup.Schedule]

func New(path string, seed backup.Schedule) *Store {
	miss := validatedSeed(seed)
	return jsonfile.New(path, "backup schedule file path is empty", miss,
		jsonfile.WithLoadHook(func(raw backup.Schedule) (backup.Schedule, error) {
			normalized, err := backup.ValidateSchedule(raw)
			if err != nil {
				// битый файл — fallback на seed, но сохраняем last_run если валидны
				out := miss
				out.LastRunAt = strings.TrimSpace(raw.LastRunAt)
				out.LastRunDate = strings.TrimSpace(raw.LastRunDate)
				return out, nil //nolint:nilerr // corrupt schedule falls back to seed
			}
			if normalized.LastRunAt == "" {
				normalized.LastRunAt = strings.TrimSpace(raw.LastRunAt)
			}
			if normalized.LastRunDate == "" {
				normalized.LastRunDate = strings.TrimSpace(raw.LastRunDate)
			}
			return normalized, nil
		}),
		jsonfile.WithSaveHook(func(st backup.Schedule) (backup.Schedule, error) {
			out, err := backup.ValidateSchedule(st)
			if err != nil {
				return backup.Schedule{}, err
			}
			out.LastRunAt = strings.TrimSpace(st.LastRunAt)
			out.LastRunDate = strings.TrimSpace(st.LastRunDate)
			out.UpdatedAt = strings.TrimSpace(st.UpdatedAt)
			return out, nil
		}),
	)
}

func validatedSeed(seed backup.Schedule) backup.Schedule {
	out, err := backup.ValidateSchedule(seed)
	if err != nil {
		return backup.DefaultsSchedule(backup.Options{Keep: 7, IncludeEdges: true, IncludeAuth: true})
	}
	return out
}
