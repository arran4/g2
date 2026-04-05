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

type UseWeakPointer bool

type LazyFileContent struct {
	weakPtr   weak.Pointer[[]byte]
	strongPtr []byte
	Loader    func() (io.ReadCloser, error)
	useWeak   bool
	mu        sync.Mutex
}

func NewLazyFileContent(loader func() (io.ReadCloser, error), opts ...any) *LazyFileContent {
	l := &LazyFileContent{
		Loader:  loader,
		useWeak: true, // Default to weak
	}
	for _, opt := range opts {
		switch o := opt.(type) {
		case UseWeakPointer:
			l.useWeak = bool(o)
		}
	}
	return l
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

	if f.useWeak {
		if p := f.weakPtr.Value(); p != nil {
			return p, nil
		}
	} else if f.strongPtr != nil {
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

	if f.useWeak {
		ptr := &b
		f.weakPtr = weak.Make(ptr)
		return ptr, nil
	} else {
		f.strongPtr = b
		return &f.strongPtr, nil
	}
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
