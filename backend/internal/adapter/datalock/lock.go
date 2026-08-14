// Package datalock — exclusive lock на control-plane data dir (single-writer).
package datalock

import (
	"fmt"
	"os"
	"path/filepath"
)

const lockName = ".nm_backend.lock"

// Lock удерживает эксклюзивную блокировку файла в dataDir.
type Lock struct {
	f *os.File
}

// Acquire берёт non-blocking exclusive lock на dataDir/.nm_backend.lock.
// Второй процесс на том же томе получит ошибку — защита от dual-writer users.json и др.
func Acquire(dataDir string) (*Lock, error) {
	dataDir = filepath.Clean(dataDir)
	if dataDir == "" || dataDir == "." {
		return nil, fmt.Errorf("datalock: empty data dir")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("datalock: mkdir %s: %w", dataDir, err)
	}
	path := filepath.Join(dataDir, lockName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("datalock: open %s: %w", path, err)
	}
	if err := lockExclusive(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("datalock: another backend already holds %s (single-writer control plane): %w", path, err)
	}
	// Best-effort PID for operators.
	_, _ = f.Seek(0, 0)
	_ = f.Truncate(0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	return &Lock{f: f}, nil
}

// Close снимает lock и закрывает файл.
func (l *Lock) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	errUnlock := unlock(l.f)
	errClose := l.f.Close()
	l.f = nil
	if errUnlock != nil {
		return errUnlock
	}
	return errClose
}
