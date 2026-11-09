package connectorsfile

import (
	"geoatlas/internal/jsonfile"
	"geoatlas/internal/usecase/connectors"
)

// New returns a JSON store for connectors document.
func New(path string) *jsonfile.Store[connectors.Document] {
	return jsonfile.New(
		path,
		"connectors file path is empty",
		connectors.Document{},
	)
}
