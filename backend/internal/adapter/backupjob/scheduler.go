package backupjob

import (
	"context"
	"sync"
	"time"

	usecasebackup "network_monitor/internal/usecase/backup"
)

// Runner — минимум для автобэкапа.
type Runner interface {
	TickAutoCreate(ctx context.Context, now time.Time)
}

// Scheduler — тик раз в минуту, вызывает TickAutoCreate.
type Scheduler struct {
	runner   Runner
	interval time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func New(runner Runner, interval time.Duration) *Scheduler {
	if interval < 15*time.Second {
		interval = time.Minute
	}
	return &Scheduler{
		runner:   runner,
		interval: interval,
		done:     make(chan struct{}),
	}
}

func NewFromService(svc *usecasebackup.Service, interval time.Duration) *Scheduler {
	return New(svc, interval)
}

func (s *Scheduler) Start(parent context.Context) {
	if s == nil {
		return
	}
	if s.runner == nil {
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
		s.runner.TickAutoCreate(ctx, time.Now().UTC())
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.runner.TickAutoCreate(ctx, time.Now().UTC())
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
