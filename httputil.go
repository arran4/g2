package g2

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/sha3"
	"io"
	"log"
	"net/http"
	"time"
)

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
	Size     int64
	Blake2b  string
	Blake2s  string
	Md5      string
	Rmd160   string
	Sha1     string
	Sha256   string
	Sha3_256 string
	Sha3_512 string
	Sha512   string
}

func DownloadAndChecksum(url string) (*Checksums, error) {
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

	blake2bHash, err := blake2b.New512(nil)
	if err != nil {
		return nil, fmt.Errorf("bad blake2b initialization: %w", err)
	}
	blake2sHash, err := blake2s.New256(nil)
	if err != nil {
		return nil, fmt.Errorf("bad blake2s initialization: %w", err)
	}
	md5Hash := md5.New()
	rmd160Hash := ripemd160.New()
	sha1Hash := sha1.New()
	sha256Hash := sha256.New()
	sha3_256Hash := sha3.New256()
	sha3_512Hash := sha3.New512()
	sha512Hash := sha512.New()

	multiWriter := io.MultiWriter(
		blake2bHash,
		blake2sHash,
		md5Hash,
		rmd160Hash,
		sha1Hash,
		sha256Hash,
		sha3_256Hash,
		sha3_512Hash,
		sha512Hash,
		&DownloadProgress{Resp: resp},
	)

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
		Size:     size,
		Blake2b:  fmt.Sprintf("%x", blake2bHash.Sum(nil)),
		Blake2s:  fmt.Sprintf("%x", blake2sHash.Sum(nil)),
		Md5:      fmt.Sprintf("%x", md5Hash.Sum(nil)),
		Rmd160:   fmt.Sprintf("%x", rmd160Hash.Sum(nil)),
		Sha1:     fmt.Sprintf("%x", sha1Hash.Sum(nil)),
		Sha256:   fmt.Sprintf("%x", sha256Hash.Sum(nil)),
		Sha3_256: fmt.Sprintf("%x", sha3_256Hash.Sum(nil)),
		Sha3_512: fmt.Sprintf("%x", sha3_512Hash.Sum(nil)),
		Sha512:   fmt.Sprintf("%x", sha512Hash.Sum(nil)),
	}

	return checksums, nil
}
