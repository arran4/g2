package g2

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/ripemd160" //nolint:staticcheck
	"golang.org/x/crypto/sha3"
	"hash"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	HashBlake2b  = "BLAKE2B"
	HashBlake2s  = "BLAKE2S"
	HashMd5      = "MD5"
	HashRmd160   = "RMD160"
	HashSha1     = "SHA1"
	HashSha256   = "SHA256"
	HashSha3_256 = "SHA3_256"
	HashSha3_512 = "SHA3_512"
	HashSha512   = "SHA512"
)

var AllHashes = []string{
	HashBlake2b,
	HashBlake2s,
	HashMd5,
	HashRmd160,
	HashSha1,
	HashSha256,
	HashSha3_256,
	HashSha3_512,
	HashSha512,
}

type DownloadProgress struct {
	Resp      *http.Response
	TotalSize *int64
	Size      int
	Tick      time.Time
	Start     time.Time
}

func (d *DownloadProgress) Write(p []byte) (n int, err error) {
	if d.Size == 0 {
		if d.Resp != nil && d.Resp.ContentLength > 0 {
			d.TotalSize = &d.Resp.ContentLength
		}
		d.Start = time.Now()
	}
	total := "unknown"
	estimate := "unknown"
	percent := "unknown%"
	t := time.Now()
	duration := t.Sub(d.Start)
	d.Size += len(p)
	if t.Sub(d.Tick) > 5*time.Second {
		d.Tick = t
		if d.TotalSize != nil {
			percent = fmt.Sprintf("%03d%%", int(100*(float64(d.Size)/float64(*d.TotalSize))))
			total = fmt.Sprintf("%d kb", *d.TotalSize/1024)
			durationInSeconds := duration / time.Second
			if durationInSeconds > 0 {
				bytesPerSec := d.Size / int(durationInSeconds)
				if bytesPerSec > 0 {
					estimate = (time.Duration(int(*d.TotalSize)/bytesPerSec) * time.Second).String()
				}
			}
		}
		log.Printf("%s %d kb / %s (%s / %s)", percent, d.Size/1024, total, duration, estimate)
	}
	return len(p), nil
}

type Checksums struct {
	Size   int64
	Hashes map[string]string
}

func DownloadAndChecksum(url string, hashes []string) (*Checksums, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("Closing HTTP error - likely incomplete data: %s", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	writers := []io.Writer{&DownloadProgress{Resp: resp}}
	hashers := make(map[string]hash.Hash)

	for _, h := range hashes {
		switch h {
		case HashBlake2b:
			blake2bHash, err := blake2b.New512(nil)
			if err != nil {
				return nil, fmt.Errorf("bad blake2b initialization: %w", err)
			}
			writers = append(writers, blake2bHash)
			hashers[HashBlake2b] = blake2bHash
		case HashBlake2s:
			blake2sHash, err := blake2s.New256(nil)
			if err != nil {
				return nil, fmt.Errorf("bad blake2s initialization: %w", err)
			}
			writers = append(writers, blake2sHash)
			hashers[HashBlake2s] = blake2sHash
		case HashMd5:
			h := md5.New()
			writers = append(writers, h)
			hashers[HashMd5] = h
		case HashRmd160:
			h := ripemd160.New()
			writers = append(writers, h)
			hashers[HashRmd160] = h
		case HashSha1:
			h := sha1.New()
			writers = append(writers, h)
			hashers[HashSha1] = h
		case HashSha256:
			h := sha256.New()
			writers = append(writers, h)
			hashers[HashSha256] = h
		case HashSha3_256:
			h := sha3.New256()
			writers = append(writers, h)
			hashers[HashSha3_256] = h
		case HashSha3_512:
			h := sha3.New512()
			writers = append(writers, h)
			hashers[HashSha3_512] = h
		case HashSha512:
			h := sha512.New()
			writers = append(writers, h)
			hashers[HashSha512] = h
		}
	}

	multiWriter := io.MultiWriter(writers...)

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
	var size int64
	select {
	case size = <-sizeCh:
	case err = <-errCh:
	}
	if err != nil {
		return nil, err
	}

	checksums := &Checksums{
		Size:   size,
		Hashes: make(map[string]string),
	}

	for k, h := range hashers {
		checksums.Hashes[k] = fmt.Sprintf("%x", h.Sum(nil))
	}

	return checksums, nil
}
