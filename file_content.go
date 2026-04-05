package g2

import (
	"io"
	"os"
	"sync"
	"weak"
)

type FileContent interface {
	Get() (*[]byte, error)
	String() string
}

type WeakFileContent struct {
	weakPtr weak.Pointer[[]byte]
	Loader  func() (io.ReadCloser, error)
	mu      sync.Mutex
}

func (f *WeakFileContent) String() string {
	b, err := f.Get()
	if err != nil || b == nil {
		return ""
	}
	return string(*b)
}

func (f *WeakFileContent) Get() (*[]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if p := f.weakPtr.Value(); p != nil {
		return p, nil
	}

	rc, err := f.Loader()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	b, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	ptr := &b
	f.weakPtr = weak.Make(ptr)
	return ptr, nil
}

type LazyFileContent struct {
	strongPtr []byte
	Loader    func() (io.ReadCloser, error)
	mu        sync.Mutex
}

func (f *LazyFileContent) String() string {
	b, err := f.Get()
	if err != nil || b == nil {
		return ""
	}
	return string(*b)
}

func (f *LazyFileContent) Get() (*[]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.strongPtr != nil {
		return &f.strongPtr, nil
	}

	rc, err := f.Loader()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	b, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	f.strongPtr = b
	return &f.strongPtr, nil
}

type MmapFileContent struct {
	path string
	mu   sync.Mutex
}

func (m *MmapFileContent) String() string {
	b, err := m.Get()
	if err != nil || b == nil {
		return ""
	}
	return string(*b)
}

func (m *MmapFileContent) Get() (*[]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := os.ReadFile(m.path)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

type MemoryFileContent struct {
	Content []byte
}

func (m *MemoryFileContent) Get() (*[]byte, error) {
	return &m.Content, nil
}

func (m *MemoryFileContent) String() string {
	return string(m.Content)
}
