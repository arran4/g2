package g2

import (
	"bytes"
	"io"
	"sync"
	"testing"
)

func TestFileContent_Data(t *testing.T) {
	generateCalls := 0
	fc := &FileContent{
		generate: func() (io.ReadCloser, error) {
			generateCalls++
			return io.NopCloser(bytes.NewBufferString("hello world")), nil
		},
	}

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

	if generateCalls != 1 {
		t.Errorf("expected exactly 1 call to generate, got %d", generateCalls)
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
