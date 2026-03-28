package g2

import (
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	mainGentooCategories map[string]bool
	mainGentooOnce       sync.Once
)

func FetchMainGentooCategories() map[string]bool {
	mainGentooOnce.Do(func() {
		mainGentooCategories = make(map[string]bool)
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("https://raw.githubusercontent.com/gentoo-mirror/gentoo/stable/profiles/categories")
		if err == nil {
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode == http.StatusOK {
				data, err := io.ReadAll(resp.Body)
				if err == nil {
					lines := strings.Split(string(data), "\n")
					for _, line := range lines {
						cat := strings.TrimSpace(line)
						if cat != "" && !strings.HasPrefix(cat, "#") {
							mainGentooCategories[cat] = true
						}
					}
				}
			}
		} else {
			log.Printf("Warning: failed to fetch main gentoo categories: %v", err)
		}
	})
	return mainGentooCategories
}
