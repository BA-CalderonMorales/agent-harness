.PHONY: build test run clean install check-remote release

BINARY_NAME=agent-harness
BUILD_DIR=./build

# Build with version info from git
# Note: Uses LOCAL git tags. Run 'check-remote' first to ensure you're building from released version.
build:
	go build -ldflags "-X main.Version=$$(git describe --tags --always --dirty) -X main.GitTag=$$(git describe --tags --exact-match 2>/dev/null || echo 'none') -X main.BuildTime=$$(date -u +%Y-%m-%d_%H:%M:%S) -X main.GitSHA=$$(git rev-parse --short HEAD)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/agent-harness

# Check remote version before building (prevents version mismatch issues)
check-remote:
	@bash scripts/release/check-remote.sh

# Release build - always checks remote first
release: check-remote build
	@echo "Build complete with verified remote version"

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
