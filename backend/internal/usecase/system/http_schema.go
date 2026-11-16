package system

import "sync"

// HTTPSchemaGate — готовность схемы, без которой /api/events не отвечает.
// Ноль (не Mark) значит «ещё не обеспечена». nil у Service — проверка выключена.
type HTTPSchemaGate struct {
	mu    sync.RWMutex
	ready bool
}

// Mark фиксирует результат EnsureHTTPSchema. Успех включает ready.
// Повторный сбой после успеха ready не сбрасывает.
func (g *HTTPSchemaGate) Mark(err error) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if err == nil {
		g.ready = true
		return
	}
	if g.ready {
		return
	}
	g.ready = false
}

// Ready — true после успешного EnsureHTTPSchema.
func (g *HTTPSchemaGate) Ready() bool {
	if g == nil {
		return true
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.ready
}
