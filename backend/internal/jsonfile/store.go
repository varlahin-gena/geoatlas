// Package jsonfile — mutex-guarded JSON control-plane file of type T.
// Atomic read/write goes through fileatomic; this package owns the lock and
// missing-file / empty-path defaults shared by retention, anomaly settings
// and backup schedule stores.
package jsonfile

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"

	"geoatlas/internal/fileatomic"
)

// Store — JSON-файл значения T. Load/Save сериализуются через RWMutex.
type Store[T any] struct {
	mu       sync.RWMutex
	path     string
	missing  T
	emptyErr string
	loadHook func(T) (T, error)
	saveHook func(T) (T, error)
}

// Option настраивает Store при создании.
type Option[T any] func(*Store[T])

// WithLoadHook вызывается после успешного Unmarshal (не при missing file).
func WithLoadHook[T any](fn func(raw T) (T, error)) Option[T] {
	return func(s *Store[T]) { s.loadHook = fn }
}

// WithSaveHook вызывается под write-lock перед атомарной записью.
func WithSaveHook[T any](fn func(T) (T, error)) Option[T] {
	return func(s *Store[T]) { s.saveHook = fn }
}

// New создаёт Store. missing возвращается при пустом path или отсутствии файла.
// emptyPathErr — текст ошибки Save при пустом path.
func New[T any](path, emptyPathErr string, missing T, opts ...Option[T]) *Store[T] {
	s := &Store[T]{
		path:     strings.TrimSpace(path),
		missing:  missing,
		emptyErr: emptyPathErr,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// Load читает JSON. Нет файла / пустой path → missing (без ошибки).
func (s *Store[T]) Load() (T, error) {
	var zero T
	if s == nil || s.path == "" {
		if s == nil {
			return zero, nil
		}
		return s.missing, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := fileatomic.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.missing, nil
		}
		return zero, err
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		return zero, err
	}
	if s.loadHook != nil {
		return s.loadHook(out)
	}
	return out, nil
}

// Save пишет JSON атомарно.
func (s *Store[T]) Save(v T) error {
	if s == nil || s.path == "" {
		msg := "jsonfile: empty path"
		if s != nil && s.emptyErr != "" {
			msg = s.emptyErr
		}
		return errors.New(msg)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveHook != nil {
		var err error
		v, err = s.saveHook(v)
		if err != nil {
			return err
		}
	}
	return fileatomic.WriteJSON(s.path, v)
}
