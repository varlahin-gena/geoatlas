package reputationsettingsfile

import (
	"geoatlas/internal/jsonfile"
	usecasereputation "geoatlas/internal/usecase/reputation"
)

// Store — JSON-файл настроек репутации (/app/data/reputation_settings.json).
type Store = jsonfile.Store[usecasereputation.Settings]

func New(path string) *Store {
	var missing usecasereputation.Settings
	return jsonfile.New(path, "reputation settings file path is empty", missing)
}
