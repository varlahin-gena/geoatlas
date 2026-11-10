package httpapi

import (
	"context"
	"net/http"
	"strings"
	"sync"
)

type mapGateEntry struct {
	gen    uint64
	cancel context.CancelFunc
}

// MapQueryGate сериализует GetMap: один in-flight запрос на ключ сессии.
// Новый Begin отменяет предыдущий context того же ключа.
type MapQueryGate struct {
	mu     sync.Mutex
	seq    uint64
	active map[string]mapGateEntry
}

func NewMapQueryGate() *MapQueryGate {
	return &MapQueryGate{active: map[string]mapGateEntry{}}
}

// Begin cancels any prior in-flight map query for key and returns a child ctx.
// Call done when the request finishes (clears the slot only if still current).
func (g *MapQueryGate) Begin(key string, parent context.Context) (ctx context.Context, done func()) {
	if g == nil {
		ctx, cancel := context.WithCancel(parent)
		return ctx, cancel
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "anon"
	}
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)

	g.mu.Lock()
	if prev, ok := g.active[key]; ok && prev.cancel != nil {
		prev.cancel()
	}
	g.seq++
	gen := g.seq
	g.active[key] = mapGateEntry{gen: gen, cancel: cancel}
	g.mu.Unlock()

	var once sync.Once
	done = func() {
		once.Do(func() {
			g.mu.Lock()
			if cur, ok := g.active[key]; ok && cur.gen == gen {
				delete(g.active, key)
			}
			g.mu.Unlock()
			cancel()
		})
	}
	return ctx, done
}

// mapQueryGateKey derives a stable per-session key for MapQueryGate.
func mapQueryGateKey(r *http.Request) string {
	if r == nil {
		return "anon"
	}
	if sess, ok := SessionFromContext(r.Context()); ok {
		if sid := strings.TrimSpace(sess.SID); sid != "" {
			return "sid:" + sid
		}
		if u := strings.TrimSpace(sess.Username); u != "" {
			return "user:" + u
		}
	}
	addr := strings.TrimSpace(r.RemoteAddr)
	if addr == "" {
		return "anon"
	}
	return "anon:" + addr
}
