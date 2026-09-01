package huntjob

import (
	"context"
	"sync"
	"time"

	"geoatlas/internal/usecase/hunts"
)

// Scheduler — минутный тик scheduled hunts.
type Scheduler struct {
	svc      *hunts.Service
	interval time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func New(svc *hunts.Service, interval time.Duration) *Scheduler {
	if interval < 15*time.Second {
		interval = time.Minute
	}
	return &Scheduler{svc: svc, interval: interval, done: make(chan struct{})}
}

func (s *Scheduler) Start(parent context.Context) {
	if s == nil || s.svc == nil {
		select {
		case <-s.done:
		default:
			close(s.done)
		}
		return
	}
	ctx, cancel := context.WithCancel(parent)
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	go func() {
		defer close(s.done)
		s.svc.TickScheduled(ctx, time.Now().UTC())
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.svc.TickScheduled(ctx, time.Now().UTC())
			}
		}
	}()
}

func (s *Scheduler) Shutdown(ctx context.Context) {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	select {
	case <-s.done:
	case <-ctx.Done():
	}
}
