package parseradapter

import (
	"context"
	"fmt"

	"geoatlas/internal/parser"
	"geoatlas/internal/usecase/parsers"
)

// Reloader loads rules from store and swaps ReloadableRegistry + samples.
type Reloader struct {
	store   StoreLister
	reg     *parser.ReloadableRegistry
	samples *SamplesHolder
}

// StoreLister — минимальный порт для Reload.
type StoreLister interface {
	List(ctx context.Context) ([]parser.RuleRecord, error)
}

func NewReloader(store StoreLister, reg *parser.ReloadableRegistry, samples *SamplesHolder) *Reloader {
	return &Reloader{store: store, reg: reg, samples: samples}
}

var _ parsers.RegistryReloader = (*Reloader)(nil)

func (r *Reloader) Reload(ctx context.Context) error {
	if r == nil || r.reg == nil {
		return fmt.Errorf("parser reloader not configured")
	}
	var rules []parser.RuleRecord
	if r.store != nil {
		list, err := r.store.List(ctx)
		if err != nil {
			return err
		}
		rules = list
	}
	built, err := parser.BuildRegistry(rules)
	if err != nil {
		return err
	}
	r.reg.Swap(built)
	if r.samples != nil {
		r.samples.Store(parser.SamplesFromRules(rules))
	}
	return nil
}
