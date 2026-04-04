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

type WeakFileContent struct {
	mu       sync.Mutex
	data     weak.Pointer[[]byte]
	generate func() (io.ReadCloser, error)
}

func (fc *WeakFileContent) Data() (*[]byte, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if ptr := fc.data.Value(); ptr != nil {
		return ptr, nil
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

	fc.data = weak.Make(&b)
	return &b, nil
}

func (fc *WeakFileContent) Close() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.data = weak.Pointer[[]byte]{}
	return nil
}

func (fc *WeakFileContent) SetGenerator(generate func() (io.ReadCloser, error)) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.generate = generate
	fc.data = weak.Pointer[[]byte]{}
}

type LazyFileContent struct {
	mu       sync.Mutex
	data     *[]byte
	generate func() (io.ReadCloser, error)
}

func (fc *LazyFileContent) Data() (*[]byte, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if fc.data != nil {
		return fc.data, nil
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

	fc.data = &b
	return fc.data, nil
}

func (fc *LazyFileContent) Close() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.data = nil
	return nil
}

func (fc *LazyFileContent) SetGenerator(generate func() (io.ReadCloser, error)) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.generate = generate
	fc.data = nil
}

type MemoryFileContent struct {
	mu       sync.Mutex
	data     *[]byte
	generate func() (io.ReadCloser, error)
}

func (fc *MemoryFileContent) load() error {
	rc, err := fc.generate()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	b, err := io.ReadAll(rc)
	if err != nil {
		return err
	}

	fc.data = &b
	return nil
}

func (fc *MemoryFileContent) Data() (*[]byte, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if fc.data == nil {
		if err := fc.load(); err != nil {
			return nil, err
		}
	}
	return fc.data, nil
}

func (fc *MemoryFileContent) Close() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.data = nil
	return nil
}

func (fc *MemoryFileContent) SetGenerator(generate func() (io.ReadCloser, error)) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.generate = generate
	fc.data = nil
	_ = fc.load() // eagerly load
}
