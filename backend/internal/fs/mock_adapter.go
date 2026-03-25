package fs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type MockAdapter struct {
	dirs         map[string][]DirEntry
	files        map[string]FileInfo
	fileContents map[string][]byte
	mu           sync.RWMutex
}

func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		dirs:         make(map[string][]DirEntry),
		files:        make(map[string]FileInfo),
		fileContents: make(map[string][]byte),
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

func (m *MockAdapter) AddFile(path string, content []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := append([]byte(nil), content...)
	m.fileContents[path] = data
	m.files[path] = FileInfo{
		Name:    filepath.Base(path),
		IsDir:   false,
		Size:    int64(len(data)),
		ModTime: time.Now().UTC(),
	}
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

	for key := range m.dirs {
		if key == path || strings.HasPrefix(key, path+string(os.PathSeparator)) {
			delete(m.dirs, key)
		}
	}
	for key := range m.files {
		if key == path || strings.HasPrefix(key, path+string(os.PathSeparator)) {
			delete(m.files, key)
			delete(m.fileContents, key)
		}
	}

	delete(m.files, path)
	delete(m.fileContents, path)
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

func (m *MockAdapter) OpenFileRead(ctx context.Context, path string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	content, ok := m.fileContents[path]
	if !ok {
		if _, exists := m.files[path]; !exists {
			return nil, fmt.Errorf("MockAdapter.OpenFileRead: %w", os.ErrNotExist)
		}
		content = nil
	}

	return io.NopCloser(bytes.NewReader(append([]byte(nil), content...))), nil
}

func (m *MockAdapter) OpenFileWrite(ctx context.Context, path string, _ os.FileMode) (io.WriteCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return &mockWriteCloser{adapter: m, path: path}, nil
}

type mockWriteCloser struct {
	adapter *MockAdapter
	path    string
	buffer  bytes.Buffer
	closed  bool
}

func (w *mockWriteCloser) Write(p []byte) (int, error) {
	if w.closed {
		return 0, fmt.Errorf("mockWriteCloser.Write: file is closed")
	}

	return w.buffer.Write(p)
}

func (w *mockWriteCloser) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	data := append([]byte(nil), w.buffer.Bytes()...)

	w.adapter.mu.Lock()
	defer w.adapter.mu.Unlock()

	w.adapter.fileContents[w.path] = data
	w.adapter.files[w.path] = FileInfo{
		Name:    filepath.Base(w.path),
		IsDir:   false,
		Size:    int64(len(data)),
		ModTime: time.Now().UTC(),
	}

	return nil
}

var _ FSAdapter = (*MockAdapter)(nil)
