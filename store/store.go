package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store[T any] interface {
	Save(items []T) error
	Load() ([]T, error)
}

type JSONFile[T any] struct {
	path string
	mu   sync.Mutex
}

func NewJSONFile[T any](path string) *JSONFile[T] {
	return &JSONFile[T]{path: path}
}

func (s *JSONFile[T]) Load() ([]T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("store: parsing %s: %w", s.path, err)
	}
	return items, nil
}

func (s *JSONFile[T]) Save(items []T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return withPermissionHint(err, dir, s.path)
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		if os.IsPermission(err) {
			if writeErr := os.WriteFile(s.path, data, 0o644); writeErr == nil {
				return nil
			}
		}
		return withPermissionHint(err, dir, s.path)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return withPermissionHint(err, dir, s.path)
	}
	return nil
}

func withPermissionHint(err error, dir, path string) error {
	if err == nil {
		return nil
	}
	if !os.IsPermission(err) {
		return err
	}
	return fmt.Errorf("%w (state persistence path %q in %q is not writable; fix volume ownership/permissions or run with matching uid/gid)", err, path, dir)
}
