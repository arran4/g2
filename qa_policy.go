package g2

import (
	"bufio"
	"io"
	"os"
	"strings"
)

type QAPolicy struct {
	Policies map[string]string
}

func ParseQAPolicy(path string) (*QAPolicy, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return ParseQAPolicyFromReader(file)
}

func ParseQAPolicyFromReader(r io.Reader) (*QAPolicy, error) {
	scanner := bufio.NewScanner(r)
	qa := &QAPolicy{
		Policies: make(map[string]string),
	}

	inPolicy := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") {
			if line == "[policy]" {
				inPolicy = true
			} else {
				inPolicy = false
			}
			continue
		}

		if inPolicy {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				if strings.HasPrefix(key, "PG") && len(key) > 2 {
					qa.Policies[key] = value
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return qa, nil
}
