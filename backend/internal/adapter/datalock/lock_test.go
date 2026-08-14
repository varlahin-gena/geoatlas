package datalock_test

import (
	"os"
	"path/filepath"
	"testing"

	"network_monitor/internal/adapter/datalock"
)

func TestAcquireExclusive(t *testing.T) {
	dir := t.TempDir()
	l1, err := datalock.Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Close()

	if _, err := datalock.Acquire(dir); err == nil {
		t.Fatal("second Acquire must fail while first holds lock")
	}
	if err := l1.Close(); err != nil {
		t.Fatal(err)
	}
	l2, err := datalock.Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer l2.Close()
	if _, err := os.Stat(filepath.Join(dir, ".nm_backend.lock")); err != nil {
		t.Fatal(err)
	}
}
