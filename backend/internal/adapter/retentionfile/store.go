package retentionfile

import (
	"geoatlas/internal/jsonfile"
	"geoatlas/internal/usecase/retention"
)

// Store — JSON-файл с TTL (том /app/data рядом с users.json).
type Store = jsonfile.Store[retention.Settings]

func New(path string) *Store {
	return jsonfile.New(path, "retention file path is empty", retention.Defaults())
}
