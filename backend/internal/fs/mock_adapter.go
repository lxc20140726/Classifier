package fs

import (
	"context"
	"fmt"
	"os"
	"sync"
)

type MockAdapter struct {
	dirs  map[string][]DirEntry
	files map[string]FileInfo
	mu    sync.RWMutex
}

func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		dirs:  make(map[string][]DirEntry),
		files: make(map[string]FileInfo),
	}
}

func (m *MockAdapter) AddDir(path string, entries []DirEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cloned := make([]DirEntry, len(entries))
	copy(cloned, entries)
	m.dirs[path] = cloned
}

func (m *MockAdapter) ReadDir(ctx context.Context, path string) ([]DirEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, ok := m.dirs[path]
	if !ok {
		return nil, fmt.Errorf("MockAdapter.ReadDir: %w", os.ErrNotExist)
	}

	cloned := make([]DirEntry, len(entries))
	copy(cloned, entries)
	return cloned, nil
}

func (m *MockAdapter) Stat(ctx context.Context, path string) (FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return FileInfo{}, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	info, ok := m.files[path]
	if !ok {
		return FileInfo{}, fmt.Errorf("MockAdapter.Stat: %w", os.ErrNotExist)
	}

	return info, nil
}

func (m *MockAdapter) MoveDir(ctx context.Context, src, dst string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entries, ok := m.dirs[src]
	if !ok {
		return fmt.Errorf("MockAdapter.MoveDir: %w", os.ErrNotExist)
	}

	cloned := make([]DirEntry, len(entries))
	copy(cloned, entries)
	m.dirs[dst] = cloned
	delete(m.dirs, src)
	return nil
}

func (m *MockAdapter) MkdirAll(ctx context.Context, path string, _ os.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.dirs[path]; !ok {
		m.dirs[path] = []DirEntry{}
	}

	return nil
}

func (m *MockAdapter) Remove(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.dirs, path)
	delete(m.files, path)
	return nil
}

func (m *MockAdapter) Exists(ctx context.Context, path string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.dirs[path]; ok {
		return true, nil
	}

	if _, ok := m.files[path]; ok {
		return true, nil
	}

	return false, nil
}

var _ FSAdapter = (*MockAdapter)(nil)
