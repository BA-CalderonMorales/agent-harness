.PHONY: help build test mutation-test run clean install fmt lint check-remote release

BINARY_NAME := agent-harness
BUILD_DIR := ./build
MAIN_PKG := ./cmd/agent-harness

VERSION := $(shell git describe --tags --always --dirty)
GIT_TAG := $(shell git describe --tags --exact-match 2>/dev/null || echo none)
BUILD_TIME := $(shell date -u +%Y-%m-%d_%H:%M:%S)
GIT_SHA := $(shell git rev-parse --short HEAD)
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.GitTag=$(GIT_TAG) -X main.BuildTime=$(BUILD_TIME) -X main.GitSHA=$(GIT_SHA)

help:
	@printf 'Agent Harness make targets\n\n'
	@printf '  make run            Start the local CLI/TUI with go run\n'
	@printf '  make build          Build %s with git version metadata\n' "$(BINARY_NAME)"
	@printf '  make test           Run the full Go test suite\n'
	@printf '  make mutation-test  Run mutation tests for behavior-critical packages\n'
	@printf '  make lint           Run golangci-lint across the repo\n'
	@printf '  make fmt            Format all Go packages\n'
	@printf '  make install        Install %s into GOPATH/GOBIN\n' "$(BINARY_NAME)"
	@printf '  make clean          Remove build artifacts\n'
	@printf '  make release        Check remote release state, then build\n'

build:
	@printf '==> Building %s\n' "$(BINARY_NAME)"
	@printf '    Version:    %s\n' "$(VERSION)"
	@printf '    Git tag:    %s\n' "$(GIT_TAG)"
	@printf '    Git SHA:    %s\n' "$(GIT_SHA)"
	@printf '    Build time: %s UTC\n' "$(BUILD_TIME)"
	@mkdir -p "$(BUILD_DIR)"
	@go build -trimpath -ldflags "$(LDFLAGS)" -o "$(BUILD_DIR)/$(BINARY_NAME)" "$(MAIN_PKG)" || { \
		status=$$?; \
		printf '\n[fail] Build failed with exit status %s.\n' "$$status"; \
		exit $$status; \
	}
	@printf '[ok] Binary ready: %s\n' "$(BUILD_DIR)/$(BINARY_NAME)"

check-remote:
	@printf '==> Checking remote release state\n'
	@bash scripts/release/check-remote.sh || { \
		status=$$?; \
		printf '\n[fail] Remote release check failed with exit status %s.\n' "$$status"; \
		exit $$status; \
	}
	@printf '[ok] Remote release state checked\n'

release: check-remote build
	@printf '[ok] Release build completed after remote verification\n'

test:
	@printf '==> Running Go tests\n'
	@go test -v ./... || { \
		status=$$?; \
		printf '\n[fail] Go tests failed with exit status %s. Review the failing package output above.\n' "$$status"; \
		exit $$status; \
	}
	@printf '[ok] Go tests passed\n'

mutation-test:
	@printf '==> Running mutation tests\n'
	@printf '    Targets: defaults from scripts/verify/mutation-test.sh\n'
	@./scripts/verify/mutation-test.sh || { \
		status=$$?; \
		printf '\n[fail] Mutation tests failed with exit status %s. Review surviving mutants or setup errors above.\n' "$$status"; \
		exit $$status; \
	}
	@printf '[ok] Mutation tests passed\n'

run:
	@printf '==> Starting Agent Harness\n'
	@printf '    Command: go run %s\n\n' "$(MAIN_PKG)"
	@go run "$(MAIN_PKG)" || { \
		status=$$?; \
		printf '\n[fail] Agent Harness exited with status %s.\n' "$$status"; \
		printf '       If the app panicked, inspect the panic/output above first.\n'; \
		exit $$status; \
	}

clean:
	@printf '==> Cleaning build artifacts\n'
	@rm -rf "$(BUILD_DIR)"
	@printf '[ok] Removed %s\n' "$(BUILD_DIR)"

install:
	@printf '==> Installing %s\n' "$(BINARY_NAME)"
	@go install "$(MAIN_PKG)" || { \
		status=$$?; \
		printf '\n[fail] Install failed with exit status %s.\n' "$$status"; \
		exit $$status; \
	}
	@printf '[ok] Installed %s\n' "$(BINARY_NAME)"

fmt:
	@printf '==> Formatting Go packages\n'
	@go fmt ./... || { \
		status=$$?; \
		printf '\n[fail] go fmt failed with exit status %s.\n' "$$status"; \
		exit $$status; \
	}
	@printf '[ok] Go packages formatted\n'

lint:
	@printf '==> Running golangci-lint\n'
	@golangci-lint run ./... || { \
		status=$$?; \
		printf '\n[fail] Lint failed with exit status %s. Review diagnostics above.\n' "$$status"; \
		exit $$status; \
	}
	@printf '[ok] Lint passed\n'

verify-structure:
	@printf '==> Checking file structure (<= 400 lines per source file)\n'
	@./scripts/verify-structure.sh

verify: verify-structure
	@printf '==> Running tests\n'
	@go test ./... || { \
		status=$$?; \
		printf '\n[fail] Tests failed with exit status %s.\n' "$$status"; \
		exit $$status; \
	}
	@printf '[ok] All checks passed\n'
