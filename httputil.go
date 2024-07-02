package g2

import (
	"crypto/sha512"
	"fmt"
	"golang.org/x/crypto/blake2b"
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

	multiWriter := io.MultiWriter(blake2bHash, sha512Hash, &DownloadProgress{Resp: resp})

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
		return 0, "", "", err
	}

	blake2bSum := fmt.Sprintf("%x", blake2bHash.Sum(nil))
	sha512Sum := fmt.Sprintf("%x", sha512Hash.Sum(nil))

	return size, blake2bSum, sha512Sum, nil
}
