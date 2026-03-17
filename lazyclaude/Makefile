BINARY := lazyclaude
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.PHONY: build test lint install clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/lazyclaude

test:
	go test -race -cover ./...

lint:
	golangci-lint run ./...

install: build
	cp bin/$(BINARY) $(GOPATH)/bin/$(BINARY)

clean:
	rm -rf bin/