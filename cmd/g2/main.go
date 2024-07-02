package main

import (
	"crypto/sha512"
	"fmt"
	"golang.org/x/crypto/blake2b"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Println("Usage: go run main.go <url> <filename> <Manifest file or dir>")
		return
	}

	url := os.Args[1]
	filename := os.Args[2]
	ebuildDir := os.Args[3]

	size, blake2bSum, sha512Sum, err := downloadAndChecksum(url)
	if err != nil {
		fmt.Printf("Error downloading and calculating checksums: %v\n", err)
		return
	}

	manifestLine := fmt.Sprintf("DIST %s %d BLAKE2B %s SHA512 %s", filename, size, blake2bSum, sha512Sum)
	manifestPath := filepath.Join(ebuildDir, "Manifest")

	err = upsertManifest(manifestPath, filename, manifestLine)
	if err != nil {
		fmt.Printf("Error updating manifest: %v\n", err)
	}
}

func downloadAndChecksum(url string) (int64, string, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, "", "", fmt.Errorf("bad status: %s", resp.Status)
	}

	blake2bHash, err := blake2b.New512(nil)
	if err != nil {
		return 0, "", "", fmt.Errorf("bad blake2b initialization: %w", err)
	}
	sha512Hash := sha512.New()

	multiWriter := io.MultiWriter(blake2bHash, sha512Hash)

	sizeCh := make(chan int64)
	errCh := make(chan error)

	go func() {
		defer close(sizeCh)
		defer close(errCh)

		size, err := io.Copy(multiWriter, resp.Body)
		if err != nil {
			errCh <- err
			return
		}

		sizeCh <- size
	}()

	size := <-sizeCh
	err = <-errCh
	if err != nil {
		return 0, "", "", err
	}

	blake2bSum := fmt.Sprintf("%x", blake2bHash.Sum(nil))
	sha512Sum := fmt.Sprintf("%x", sha512Hash.Sum(nil))

	return size, blake2bSum, sha512Sum, nil
}

func upsertManifest(manifestPath, filename, manifestLine string) error {
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(manifestPath, []byte(manifestLine+"\n"), 0644)
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.Contains(line, "DIST "+filename+" ") {
			lines[i] = manifestLine
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, manifestLine)
	}

	return os.WriteFile(manifestPath, []byte(strings.Join(lines, "\n")), 0644)
}
