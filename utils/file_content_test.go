package utils

import (
	"bytes"
	"io"
	"sync"
	"testing"
)

func testFileContentImpl(t *testing.T, fc FileContent, generateCallsPtr *int) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := fc.Data()
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if string(*b) != "hello world" {
				t.Errorf("expected 'hello world', got '%s'", string(*b))
			}
		}()
	}
	wg.Wait()

	if *generateCallsPtr < 1 {
		t.Errorf("expected at least 1 call to generate, got %d", *generateCallsPtr)
	}

	// Test SetGenerator and Close
	fc.SetGenerator(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewBufferString("new world")), nil
	})

	b, err := fc.Data()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if string(*b) != "new world" {
		t.Errorf("expected 'new world', got '%s'", string(*b))
	}

	err = fc.Close()
	if err != nil {
		t.Errorf("expected no error from Close, got %v", err)
	}
}

func TestFileContent_LazyWeak(t *testing.T) {
	generateCalls := 0
	fc := NewFileContent(func() (io.ReadCloser, error) {
		generateCalls++
		return io.NopCloser(bytes.NewBufferString("hello world")), nil
	}, UseWeakStorage(true), UseLazyLoading(true))
	testFileContentImpl(t, fc, &generateCalls)
}

func TestFileContent_LazyMemory(t *testing.T) {
	generateCalls := 0
	fc := NewFileContent(func() (io.ReadCloser, error) {
		generateCalls++
		return io.NopCloser(bytes.NewBufferString("hello world")), nil
	}, UseMemoryStorage(true), UseLazyLoading(true))
	testFileContentImpl(t, fc, &generateCalls)
}

func TestFileContent_EagerWeak(t *testing.T) {
	generateCalls := 0
	fc := NewFileContent(func() (io.ReadCloser, error) {
		generateCalls++
		return io.NopCloser(bytes.NewBufferString("hello world")), nil
	}, UseWeakStorage(true), UseEagerLoading(true))
	testFileContentImpl(t, fc, &generateCalls)
}

func TestFileContent_EagerMemory(t *testing.T) {
	generateCalls := 0
	fc := NewFileContent(func() (io.ReadCloser, error) {
		generateCalls++
		return io.NopCloser(bytes.NewBufferString("hello world")), nil
	}, UseMemoryStorage(true), UseEagerLoading(true))
	testFileContentImpl(t, fc, &generateCalls)
}
