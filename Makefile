VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build install clean test lint

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/gpulab ./cmd/gpulab

install: build
	cp bin/gpulab /usr/local/bin/gpulab

clean:
	rm -rf bin/

test:
	go test ./...

lint:
	go vet ./...

# Cross-compile for all platforms
build-all:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/gpulab-darwin-amd64 ./cmd/gpulab
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/gpulab-darwin-arm64 ./cmd/gpulab
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/gpulab-linux-amd64 ./cmd/gpulab
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/gpulab-linux-arm64 ./cmd/gpulab
