package huntjob

import (
	"context"
	"testing"
	"time"
)

// Start на nil-приёмнике не должен паниковать.
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s.Shutdown(ctx)
	if ctx.Err() != nil {
		t.Fatal("Shutdown timed out; done was not closed when service is nil")
	}
}
