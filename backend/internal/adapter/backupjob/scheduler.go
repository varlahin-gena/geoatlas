package backupjob

import (
	"context"
	"time"

	"geoatlas/internal/jobscheduler"
	usecasebackup "geoatlas/internal/usecase/backup"
)

// Runner — минимум для автобэкапа.
type Runner interface {
	TickAutoCreate(ctx context.Context, now time.Time)
}

// Scheduler — тик раз в минуту, вызывает TickAutoCreate.
type Scheduler struct {
	runner   Runner
	interval time.Duration
	loop     *jobscheduler.Loop
}

func New(runner Runner, interval time.Duration) *Scheduler {
	if interval < 15*time.Second {
		interval = time.Minute
	}
	return &Scheduler{
		runner:   runner,
		interval: interval,
		loop:     jobscheduler.NewLoop(),
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
		s.loop.CloseDone()
		return
	}
	s.loop.Start(parent, s.interval, func(ctx context.Context) {
		s.runner.TickAutoCreate(ctx, time.Now().UTC())
	})
}

func (s *Scheduler) Shutdown(ctx context.Context) {
	if s == nil {
		return
	}
	s.loop.Shutdown(ctx)
}
