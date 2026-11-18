package parser

import (
	"sync"
	"sync/atomic"

	"geoatlas/internal/model"
)

// ParseVerboseFunc — функциональный порт для dry-run / тестов.
type ParseVerboseFunc func(line string) ParseResult

// ReloadableRegistry — atomic snapshot *Registry для ingest / parse-test.
type ReloadableRegistry struct {
	cur atomic.Pointer[Registry]
	mu  sync.Mutex
}

// NewReloadable wraps an initial registry (nil → empty).
func NewReloadable(reg *Registry) *ReloadableRegistry {
	r := &ReloadableRegistry{}
	if reg == nil {
		reg = NewRegistry()
	}
	r.cur.Store(reg)
	return r
}

// Get returns the current registry snapshot (never nil).
func (r *ReloadableRegistry) Get() *Registry {
	if r == nil {
		return NewRegistry()
	}
	if reg := r.cur.Load(); reg != nil {
		return reg
	}
	return NewRegistry()
}

// Swap atomically replaces the registry.
func (r *ReloadableRegistry) Swap(reg *Registry) {
	if r == nil {
		return
	}
	if reg == nil {
		reg = NewRegistry()
	}
	r.mu.Lock()
	r.cur.Store(reg)
	r.mu.Unlock()
}

// ParseVerbose delegates to the current snapshot.
func (r *ReloadableRegistry) ParseVerbose(line string) ParseResult {
	return r.Get().ParseVerbose(line)
}

// Parse delegates to the current snapshot.
func (r *ReloadableRegistry) Parse(line string) (model.TrafficLog, bool) {
	return r.Get().Parse(line)
}
