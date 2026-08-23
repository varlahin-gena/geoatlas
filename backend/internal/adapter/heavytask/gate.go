package heavytask

import "sync"

// Skipper — причина пропустить тяжёлую работу ("" = можно).
type Skipper interface {
	SkipReason() string
}

// DeferredGate — Skipper, который можно привязать после старта ingest.
type DeferredGate struct {
	mu    sync.RWMutex
	inner Skipper
}

// NewDeferredGate создаёт пустой gate (SkipReason всегда "").
func NewDeferredGate() *DeferredGate {
	return &DeferredGate{}
}

// Set подменяет inner (обычно anomalyjob.Gate{Ingest: ...}).
func (g *DeferredGate) Set(inner Skipper) {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.inner = inner
	g.mu.Unlock()
}

// SkipReason делегирует inner; nil-safe.
func (g *DeferredGate) SkipReason() string {
	if g == nil {
		return ""
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.inner == nil {
		return ""
	}
	return g.inner.SkipReason()
}
