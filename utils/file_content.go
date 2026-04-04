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

type LazyFileContent struct {
	mu       sync.Mutex
	store    BytesStore
	generate func() (io.ReadCloser, error)
}

func NewLazyFileContent(store BytesStore, generate func() (io.ReadCloser, error)) *LazyFileContent {
	return &LazyFileContent{
		store:    store,
		generate: generate,
	}
}

func (fc *LazyFileContent) Data() (*[]byte, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if val := fc.store.Get(); val != nil {
		return val, nil
	}

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

func (fc *LazyFileContent) Close() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.store.Clear()
	return nil
}

func (fc *LazyFileContent) SetGenerator(generate func() (io.ReadCloser, error)) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.generate = generate
	fc.store.Clear()
}

type EagerFileContent struct {
	mu       sync.Mutex
	store    BytesStore
	generate func() (io.ReadCloser, error)
}

func NewEagerFileContent(store BytesStore, generate func() (io.ReadCloser, error)) *EagerFileContent {
	fc := &EagerFileContent{
		store:    store,
		generate: generate,
	}
	_ = fc.load()
	return fc
}

func (fc *EagerFileContent) load() error {
	rc, err := fc.generate()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	b, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	fc.store.Set(&b)
	return nil
}

func (fc *EagerFileContent) Data() (*[]byte, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if fc.store.Get() == nil {
		if err := fc.load(); err != nil {
			return nil, err
		}
	}
	return fc.store.Get(), nil
}

func (fc *EagerFileContent) Close() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.store.Clear()
	return nil
}

func (fc *EagerFileContent) SetGenerator(generate func() (io.ReadCloser, error)) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.generate = generate
	fc.store.Clear()
	_ = fc.load() // eagerly load
}
