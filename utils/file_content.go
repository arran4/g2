package utils

import (
	"io"
	"sync"
	"weak"
)

type FileContent interface {
	Data() (*[]byte, error)
	Close() error
	SetGenerator(func() (io.ReadCloser, error))
}

type BytesStore interface {
	Get() *[]byte
	Set(*[]byte)
	Clear()
}

type WeakBytesStore struct {
	ptr weak.Pointer[[]byte]
}

func (s *WeakBytesStore) Get() *[]byte {
	return s.ptr.Value()
}

func (s *WeakBytesStore) Set(val *[]byte) {
	if val == nil {
		s.ptr = weak.Pointer[[]byte]{}
	} else {
		s.ptr = weak.Make(val)
	}
}

func (s *WeakBytesStore) Clear() {
	s.ptr = weak.Pointer[[]byte]{}
}

type MemoryBytesStore struct {
	val *[]byte
}

func (s *MemoryBytesStore) Get() *[]byte {
	return s.val
}

func (s *MemoryBytesStore) Set(val *[]byte) {
	s.val = val
}

func (s *MemoryBytesStore) Clear() {
	s.val = nil
}

type UseWeakStorage bool
type UseMemoryStorage bool
type UseLazyLoading bool
type UseEagerLoading bool

type fileContentConfig struct {
	store BytesStore
	lazy  bool
}

type defaultFileContent struct {
	mu       sync.Mutex
	store    BytesStore
	lazy     bool
	generate func() (io.ReadCloser, error)
}

func NewFileContent(generate func() (io.ReadCloser, error), opts ...any) FileContent {
	cfg := fileContentConfig{
		store: &MemoryBytesStore{},
		lazy:  true,
	}

	for _, opt := range opts {
		switch o := opt.(type) {
		case UseWeakStorage:
			if o {
				cfg.store = &WeakBytesStore{}
			}
		case UseMemoryStorage:
			if o {
				cfg.store = &MemoryBytesStore{}
			}
		case UseLazyLoading:
			if o {
				cfg.lazy = true
			}
		case UseEagerLoading:
			if o {
				cfg.lazy = false
			}
		}
	}

	fc := &defaultFileContent{
		store:    cfg.store,
		lazy:     cfg.lazy,
		generate: generate,
	}

	if !fc.lazy {
		_, _ = fc.load()
	}

	return fc
}

func (fc *defaultFileContent) load() (*[]byte, error) {
	rc, err := fc.generate()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	b, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	fc.store.Set(&b)
	return &b, nil
}

func (fc *defaultFileContent) Data() (*[]byte, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if val := fc.store.Get(); val != nil {
		return val, nil
	}

	val, err := fc.load()
	if err != nil {
		return nil, err
	}

	return val, nil
}

func (fc *defaultFileContent) Close() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.store.Clear()
	return nil
}

func (fc *defaultFileContent) SetGenerator(generate func() (io.ReadCloser, error)) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.generate = generate
	fc.store.Clear()
	if !fc.lazy {
		_, _ = fc.load()
	}
}
