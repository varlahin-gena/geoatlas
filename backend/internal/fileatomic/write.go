// Package fileatomic — crash-safe replace of a regular file (tmp + fsync + rename).
// Used by the JSON control plane on /app/data (single-writer, exclusive lock).
package fileatomic

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// WriteFile writes data to path atomically: temp file, fsync, rename over dest.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("fileatomic: empty path")
	}
	if perm == 0 {
		perm = 0o600
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm) //nolint:gosec // G304: caller path is control-plane file
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := replace(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	syncDir(dir)
	return nil
}

// WriteJSON marshals v with indent and a trailing newline, then WriteFile 0600.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return WriteFile(path, data, 0o600)
}

func replace(tmp, dest string) error {
	err := os.Rename(tmp, dest)
	if err == nil {
		return nil
	}
	// Windows cannot rename over an existing file.
	if rmErr := os.Remove(dest); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmp, dest)
}

func syncDir(dir string) {
	d, err := os.Open(dir) //nolint:gosec // G304: parent of the file we just wrote
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
