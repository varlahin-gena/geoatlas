// Package fileatomic — crash-safe replace of a regular file (tmp + fsync + rename).
// Used by the JSON control plane on /app/data. Writers to the same path are safe
// to run concurrently: each gets its own temp file and the rename picks a winner.
package fileatomic

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// staleTempAge — порог, после которого tmp считается осиротевшим (падение
// процесса между CreateTemp и rename). Заведомо больше одной записи, чтобы
// уборка не тронула файл параллельного писателя.
const staleTempAge = time.Hour

// Бюджет ожидания, пока параллельный читатель отпустит целевой файл (Windows).
const (
	replaceAttempts   = 25
	replaceRetryDelay = 2 * time.Millisecond
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
	base := filepath.Base(path)
	sweepStaleTemps(dir, base)
	// Уникальный tmp: фиксированное имя path+".tmp" с O_TRUNC позволяло двум
	// писателям в один путь обрезать файл друг друга и переименовать обрывок.
	f, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := f.Chmod(perm); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
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

// ReadFile — чтение файла, который параллельно может атомарно заменяться.
// На Unix это обычный os.ReadFile. На Windows открытие падает с sharing
// violation, пока идёт MoveFileEx, поэтому коротко повторяем.
// Отсутствие файла возвращаем сразу: «файла нет» — штатный ответ для сторов.
func ReadFile(path string) ([]byte, error) {
	var (
		data []byte
		err  error
	)
	for attempt := range replaceAttempts {
		data, err = os.ReadFile(path) //nolint:gosec // G304: caller path is control-plane file
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return data, err
		}
		if attempt < replaceAttempts-1 {
			time.Sleep(replaceRetryDelay)
		}
	}
	return data, err
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

// replace переносит tmp на dest. На Unix первый Rename всегда успешен.
// На Windows MoveFileEx падает с sharing violation, пока dest открыт читателем
// (Go не выставляет FILE_SHARE_DELETE), поэтому коротко повторяем — иначе
// откат Remove+Rename оставляет окно, в котором файла не существует и
// параллельный Load() молча получает дефолты.
func replace(tmp, dest string) error {
	var err error
	for attempt := range replaceAttempts {
		if err = os.Rename(tmp, dest); err == nil {
			return nil
		}
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if attempt < replaceAttempts-1 {
			time.Sleep(replaceRetryDelay)
		}
	}
	// Последняя попытка: снять dest и переименовать (не атомарно).
	if rmErr := os.Remove(dest); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmp, dest)
}

// sweepStaleTemps — best-effort уборка осиротевших tmp рядом с целевым файлом.
// Учитывает и legacy-имя base+".tmp" от прежней реализации.
func sweepStaleTemps(dir, base string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := base + ".tmp-"
	legacy := base + ".tmp"
	cutoff := time.Now().Add(-staleTempAge)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || (name != legacy && !strings.HasPrefix(name, prefix)) {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}

func syncDir(dir string) {
	d, err := os.Open(dir) //nolint:gosec // G304: parent of the file we just wrote
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
