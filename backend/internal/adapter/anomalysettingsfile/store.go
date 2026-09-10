package anomalysettingsfile

import (
	"geoatlas/internal/jsonfile"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
)

// Store — JSON-файл настроек движка аномалий (/app/data/anomaly_settings.json).
type Store = jsonfile.Store[usecaseanomaly.Settings]

func New(path string) *Store {
	var missing usecaseanomaly.Settings
	return jsonfile.New(path, "anomaly settings file path is empty", missing)
}
