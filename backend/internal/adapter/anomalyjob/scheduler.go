package anomalyjob

import (
	"context"
	"sync"
	"time"

	"geoatlas/internal/adapter/clickhouse/aggstate"
	"geoatlas/internal/adapter/heavytask"
	"geoatlas/internal/adapter/ingestnet"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
)

// Scheduler периодически запускает Scan.
type Scheduler struct {
	svc      *usecaseanomaly.Service
	interval time.Duration
	delay    time.Duration
	heavy    *heavytask.Limiter

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	busy   sync.Mutex
	ticker *time.Ticker
}

// SetLimiter — общий слот тяжёлых задач; при занятости тик пропускается.
func (s *Scheduler) SetLimiter(l *heavytask.Limiter) {
	if s == nil {
		return
	}
	s.heavy = l
}

func New(svc *usecaseanomaly.Service, interval, startDelay time.Duration) *Scheduler {
	if interval < time.Minute {
		interval = 5 * time.Minute
	}
	if startDelay < 0 {
		startDelay = time.Minute
	}
	return &Scheduler{
		svc:      svc,
		interval: interval,
		delay:    startDelay,
		done:     make(chan struct{}),
	}
}

func (s *Scheduler) SetInterval(d time.Duration) {
	if s == nil {
		return
	}
	if d < time.Minute {
		d = time.Minute
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interval = d
	if s.ticker != nil {
		s.ticker.Reset(d)
	}
}

func (s *Scheduler) Start(parent context.Context) {
	if s == nil {
		return
	}
	if s.svc == nil || !s.svc.Available() {
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
		if s.delay > 0 {
			t := time.NewTimer(s.delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
		}
		s.tick(ctx)
		s.mu.Lock()
		s.ticker = time.NewTicker(s.interval)
		ticker := s.ticker
		s.mu.Unlock()
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
}

func (s *Scheduler) tick(ctx context.Context) {
	if !s.busy.TryLock() {
		return
	}
	defer s.busy.Unlock()
	if !s.heavy.TryAcquire() {
		return
	}
	defer s.heavy.Release()
	s.svc.Scan(ctx, time.Now().UTC())
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

// Gate — SkipReason для circuit + edges rebuild.
type Gate struct {
	Ingest *ingestnet.Service
}

func (g Gate) SkipReason() string {
	if g.Ingest != nil && g.Ingest.Stats().CircuitOpen {
		return "circuit"
	}
	st := aggstate.GetEdgesAggStatus()
	if st.State == "running" {
		return "rebuild"
	}
	return ""
}
