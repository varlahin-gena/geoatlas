package huntjob

import (
	"context"
	"testing"
	"time"
)

// Start на nil-приёмнике не должен разыменовывать s.done.
func TestSchedulerStartNilReceiver(t *testing.T) {
	var s *Scheduler
	s.Start(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s.Shutdown(ctx)
}

// Без сервиса Start закрывает done, чтобы Shutdown не ждал таймаут.
func TestSchedulerStartWithoutServiceClosesDone(t *testing.T) {
	s := New(nil, time.Minute)
	s.Start(context.Background())

	select {
	case <-s.done:
	case <-time.After(time.Second):
		t.Fatal("done not closed when service is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s.Shutdown(ctx)
}
