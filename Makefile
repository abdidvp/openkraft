.PHONY: build test lint

VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS := -ldflags "-X github.com/openkraft/openkraft/internal/cli.version=$(VERSION) -X github.com/openkraft/openkraft/internal/cli.commit=$(COMMIT)"

build:
	go build $(LDFLAGS) -o bin/openkraft ./cmd/openkraft

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

install:
	go install $(LDFLAGS) ./cmd/openkraft
