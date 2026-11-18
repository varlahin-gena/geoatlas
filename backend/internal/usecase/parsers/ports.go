package parsers

import (
	"context"

	"geoatlas/internal/parser"
)

// Store — persistence for editable parser rules.
type Store interface {
	List(ctx context.Context) ([]parser.RuleRecord, error)
	Upsert(ctx context.Context, rec parser.RuleRecord) error
	Delete(ctx context.Context, id string) error
	Exists(ctx context.Context, id string) (bool, error)
}

// RegistryReloader rebuilds the live parse registry from DB rules.
type RegistryReloader interface {
	Reload(ctx context.Context) error
}
