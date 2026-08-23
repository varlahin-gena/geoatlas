package backup

import (
	"testing"
	"time"
)

func TestJobTryStartBusy(t *testing.T) {
	j := NewJob()
	if !j.TryStart() {
		t.Fatal("first start")
	}
	if j.TryStart() {
		t.Fatal("second start must fail")
	}
	j.SetOK("ga-x", "done")
	j.Finish()
	// After finish, allow next.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if j.TryStart() {
			j.Finish()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("expected TryStart after Finish")
}
