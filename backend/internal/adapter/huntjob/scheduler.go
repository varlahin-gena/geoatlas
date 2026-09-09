package huntjob

import (
	"context"
	"time"

	"geoatlas/internal/jobscheduler"
	"geoatlas/internal/usecase/hunts"
)

// Scheduler — минутный тик scheduled hunts.
type Scheduler struct {
	svc      *hunts.Service
	interval time.Duration
	loop     *jobscheduler.Loop
}

func New(svc *hunts.Service, interval time.Duration) *Scheduler {
	if interval < 15*time.Second {
		interval = time.Minute
	}
	return &Scheduler{svc: svc, interval: interval, loop: jobscheduler.NewLoop()}
}

func (s *Scheduler) Start(parent context.Context) {
	if s == nil {
		return
	}
	if s.svc == nil {
		s.loop.CloseDone()
		return
	}
	s.loop.Start(parent, s.interval, func(ctx context.Context) {
		s.svc.TickScheduled(ctx, time.Now().UTC())
	})
}

func (s *Scheduler) Shutdown(ctx context.Context) {
	if s == nil {
		return
	}
	s.loop.Shutdown(ctx)
}
