package ingest

import (
	"sync"
	"testing"
	"time"
)

func TestStatsAggregatedWorkerErrorsNoFlicker(t *testing.T) {
	st := newStats()
	st.setState("running")

	// Два worker'а в ошибке: успех одного не снимает error state.
	st.noteWorkerError("worker-a")
	st.noteWorkerError("worker-b")
	if st.snapshot().State != "error" {
		t.Fatalf("state=%s want error", st.snapshot().State)
	}
	st.noteWorkerOK() // один ожил
	if st.snapshot().State != "error" {
		t.Fatalf("one worker still failing: state=%s", st.snapshot().State)
	}
	if st.snapshot().LastError == "" {
		t.Fatal("last_error should remain while any worker failing")
	}
	st.noteWorkerOK() // оба ожили
	snap := st.snapshot()
	if snap.State != "running" {
		t.Fatalf("state=%s want running", snap.State)
	}
	if snap.LastError != "" {
		t.Fatalf("last_error=%q want empty", snap.LastError)
	}
}

func TestStatsConcurrentNoteWorkers(t *testing.T) {
	st := newStats()
	st.setState("running")
	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			st.noteWorkerError("x")
			time.Sleep(time.Millisecond)
			st.noteWorkerOK()
		}()
	}
	wg.Wait()
	snap := st.snapshot()
	if snap.State != "running" {
		t.Fatalf("state=%s activeErrors=%d", snap.State, st.activeErrors.Load())
	}
	if st.activeErrors.Load() != 0 {
		t.Fatalf("activeErrors=%d want 0", st.activeErrors.Load())
	}
}
