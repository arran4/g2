.PHONY: readme test build

readme:
	go run ./cmd/g2 readme-gen

test:
	go test ./...

build:
	go build ./...
