.PHONY: build test run clean install

BINARY_NAME=agent-harness
BUILD_DIR=./build

build:
	go build -ldflags "-X main.Version=$$(git describe --tags --always --dirty) -X main.GitTag=$$(git describe --tags --exact-match 2>/dev/null || echo 'none') -X main.BuildTime=$$(date -u +%Y-%m-%d_%H:%M:%S) -X main.GitSHA=$$(git rev-parse --short HEAD)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/agent-harness

test:
	go test -v ./...

run:
	go run ./cmd/agent-harness

clean:
	rm -rf $(BUILD_DIR)

install:
	go install ./cmd/agent-harness

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...
