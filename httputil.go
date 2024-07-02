package g2

import (
	"crypto/sha512"
	"fmt"
	"golang.org/x/crypto/blake2b"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func DownloadAndChecksum(url string) (int64, string, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, "", "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Closing HTTP error - likely incomplete data: %s", err)
		}
	}(resp.Body)

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

func UpsertManifest(manifestPath, filename, manifestLine string) error {
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
