// Package heavytask — общий слот для тяжёлых фоновых задач (geo/backup/anomaly/reputation).
// Один процесс, один cgroup: не даём CH/CPU-работам накладываться друг на друга.
package heavytask

import (
	"context"
	"sync/atomic"
)

// Limiter — counting semaphore (обычно n=1).
type Limiter struct {
	sem  chan struct{}
	busy atomic.Bool
}

// New создаёт limiter. n<=0 → 1.
func New(n int) *Limiter {
	if n <= 0 {
		n = 1
	}
	return &Limiter{sem: make(chan struct{}, n)}
}

// TryAcquire занимает слот без ожидания. nil-receiver = всегда ok.
func (l *Limiter) TryAcquire() bool {
	if l == nil {
		return true
	}
	select {
	case l.sem <- struct{}{}:
		l.busy.Store(true)
		return true
	default:
		return false
	}
}

// Acquire ждёт слот или отмены ctx.
func (l *Limiter) Acquire(ctx context.Context) error {
	if l == nil {
		return nil
	}
	select {
	case l.sem <- struct{}{}:
		l.busy.Store(true)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release освобождает слот. Лишний Release на nil/пустом — no-op.
func (l *Limiter) Release() {
	if l == nil {
		return
	}
	select {
	case <-l.sem:
		if len(l.sem) == 0 {
			l.busy.Store(false)
		}
	default:
	}
}

// Busy — занят ли хотя бы один слот (для ingest degrade / метрик).
func (l *Limiter) Busy() bool {
	if l == nil {
		return false
	}
	return l.busy.Load()
}
