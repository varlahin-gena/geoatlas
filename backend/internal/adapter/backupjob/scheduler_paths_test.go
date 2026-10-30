package backupjob

import (
	"context"
	"testing"
	"time"

	usecasebackup "geoatlas/internal/usecase/backup"
)

func TestNewClampsNilAndFromService(t *testing.T) {
	s := New(nil, time.Millisecond)
	if s.interval < 15*time.Second {
		t.Fatal(s.interval)
	}
	var nilS *Scheduler
	nilS.Start(context.Background())
	nilS.Shutdown(context.Background())

	s.Start(context.Background()) // nil runner closes done
	s.Shutdown(context.Background())

	_ = NewFromService((*usecasebackup.Service)(nil), time.Minute)
}
