package backup

import (
	"context"
	"sync"
	"time"
)

// Job — сериализованный статус одной backup-задачи.
type Job struct {
	mu     sync.Mutex
	busy   bool
	status Status
	done   chan struct{}
}

func NewJob() *Job {
	return &Job{
		status: Status{State: "idle", Message: "not started"},
		done:   make(chan struct{}),
	}
}

func (j *Job) Status() Status {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status
}

func (j *Job) TryStart() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.busy {
		return false
	}
	j.busy = true
	j.done = make(chan struct{})
	now := time.Now().UTC()
	j.status = Status{State: "running", Message: "starting", StartedAt: now, UpdatedAt: now}
	return true
}

func (j *Job) SetRunning(name, msg string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status.State = "running"
	j.status.Name = name
	j.status.Message = msg
	j.status.UpdatedAt = time.Now().UTC()
}

func (j *Job) SetOK(name, msg string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status.State = "ok"
	j.status.Name = name
	j.status.Message = msg
	j.status.UpdatedAt = time.Now().UTC()
}

func (j *Job) SetError(name, msg string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status.State = "error"
	j.status.Name = name
	j.status.Message = msg
	j.status.UpdatedAt = time.Now().UTC()
}

func (j *Job) Finish() {
	j.mu.Lock()
	j.busy = false
	ch := j.done
	j.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (j *Job) Wait(ctx context.Context) {
	j.mu.Lock()
	busy := j.busy
	ch := j.done
	j.mu.Unlock()
	if !busy || ch == nil {
		return
	}
	select {
	case <-ch:
	case <-ctx.Done():
	}
}
