// Package jobscheduler — общий фоновый цикл «сразу + ticker» для job-адаптеров.
package jobscheduler

import (
	"context"
	"sync"
	"time"
)

// Loop владеет cancel/done горутины тикера.
type Loop struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewLoop создаёт Loop с открытым done-каналом.
func NewLoop() *Loop {
	return &Loop{done: make(chan struct{})}
}

// Done закрывается, когда горутина тикера завершилась (или после CloseDone).
func (l *Loop) Done() <-chan struct{} {
	if l == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return l.done
}

// CloseDone помечает цикл завершённым без запуска (nil runner / disabled).
// Идемпотентно.
func (l *Loop) CloseDone() {
	if l == nil {
		return
	}
	select {
	case <-l.done:
	default:
		close(l.done)
	}
}

// Start вызывает fn сразу, затем каждые interval, пока parent не отменён.
// Второй Start перезаписывает cancel; done закрывается один раз — не вызывайте
// Start повторно на том же Loop после завершения.
func (l *Loop) Start(parent context.Context, interval time.Duration, fn func(ctx context.Context)) {
	if l == nil || fn == nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	l.mu.Lock()
	l.cancel = cancel
	l.mu.Unlock()
	go func() {
		defer close(l.done)
		fn(ctx)
		if ctx.Err() != nil {
			return
		}
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				fn(ctx)
			}
		}
	}()
}

// Shutdown отменяет цикл и ждёт done либо дедлайн ctx.
func (l *Loop) Shutdown(ctx context.Context) {
	if l == nil {
		return
	}
	l.mu.Lock()
	cancel := l.cancel
	l.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	select {
	case <-l.done:
	case <-ctx.Done():
	}
}
