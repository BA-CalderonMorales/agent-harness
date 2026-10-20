.PHONY: help build test mutation-test run clean install fmt lint check-remote release verify-structure verify complexity

BINARY_NAME := agent-harness
BUILD_DIR := ./build
RUN_ARGS ?=
MUTATION_TARGETS ?=
COMPLEXITY_ARGS ?=

ifeq ($(OS),Windows_NT)
# PowerShell can invoke native GNU Make without adding Git's Unix tools to
# the user's PATH. GIT_BASH_ROOT can override a nonstandard Git install.
GIT_WINDOWS_EXE := $(subst \,/,$(shell where.exe git.exe))
GIT_BASH_ROOT ?= $(subst /cmd/git.exe,,$(GIT_WINDOWS_EXE))
SHELL := $(GIT_BASH_ROOT)/bin/bash.exe
export PATH := $(GIT_BASH_ROOT)/usr/bin;$(PATH)
BINARY_NAME := agent-harness.exe
TEST_GUARD := powershell.exe -NoProfile -File scripts/verify/memguard.ps1
else
TEST_GUARD := bash scripts/verify/memguard.sh
endif

# Leave room for compilers and the OS on an 8GB development machine.
# These soft Go runtime limits remain configurable through the environment.
export GOMEMLIMIT ?= 2GiB
export GOGC ?= 100

# Hard RSS ceiling for test runs. GOMEMLIMIT is soft — a live-heap leak
# walks straight past it (tui.test did, twice) — so the memguard kills
# the run before the kernel's OOM killer takes the host.
MEMGUARD_MB ?= 8192
MAIN_PKG := ./cmd/agent-harness

# VERSION comes from the Version constant in main.go — the same line
# bump-version.yml rewrites — so a build always reports the release line
# it belongs to. git describe cannot do this job: tags live on main, so
# develop and release/* builds describe against an older tag and lie
# about what is running.
APP_VERSION := $(shell sed -n 's/.*Version[[:space:]]*= "\([^"]*\)".*/\1/p' cmd/agent-harness/main.go)
DIRTY_SUFFIX := $(shell git diff-index --quiet HEAD 2>/dev/null || printf dirty)
VERSION := $(APP_VERSION)$(if $(DIRTY_SUFFIX),-$(DIRTY_SUFFIX))
GIT_TAG := $(shell git describe --tags --exact-match 2>/dev/null || echo none)
BUILD_TIME := $(shell date -u +%Y-%m-%d_%H:%M:%S)
GIT_SHA := $(shell git rev-parse --short HEAD)
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.GitTag=$(GIT_TAG) -X main.BuildTime=$(BUILD_TIME) -X main.GitSHA=$(GIT_SHA)

help:
	@printf 'Agent Harness make targets\n\n'
	@printf '  make run            Build (if stale) and run the built binary\n'
	@printf '  make build          Build %s with git version metadata\n' "$(BINARY_NAME)"
	@printf '  make test           Run the full Go test suite\n'
	@printf '  make mutation-test  Run mutation tests for behavior-critical packages\n'
	@printf '  make lint           Run golangci-lint across the repo\n'
	@printf '  make fmt            Format all Go packages\n'
	@printf '  make install        Install %s into GOPATH/GOBIN\n' "$(BINARY_NAME)"
	@printf '  make clean          Remove build artifacts\n'
	@printf '  make release        Check remote release state, then build\n'
	@printf '  make verify         Run structure check and the full Go test suite\n'
	@printf '  make complexity     Report function and package complexity hot spots\n'

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
	@printf '    GOMEMLIMIT: %s (soft heap goal)\n' "$(GOMEMLIMIT)"
	@printf '    memguard:   %sMB RSS ceiling (hard OOM guard)\n' "$(MEMGUARD_MB)"
	@$(TEST_GUARD) "$(MEMGUARD_MB)" go test -v -timeout 15m ./... || { \
		status=$$?; \
		printf '\n[fail] Go tests failed with exit status %s. Review the failing package output above.\n' "$$status"; \
		exit $$status; \
	}
	@printf '[ok] Go tests passed\n'

mutation-test:
	@printf '==> Running mutation tests\n'
	@printf '    Targets: defaults from scripts/verify/mutation-test.sh\n'
	@bash scripts/verify/mutation-test.sh $(MUTATION_TARGETS) || { \
		status=$$?; \
		printf '\n[fail] Mutation tests failed with exit status %s. Review surviving mutants or setup errors above.\n' "$$status"; \
		exit $$status; \
	}
	@printf '[ok] Mutation tests passed\n'

run: build
	@printf '==> Starting Agent Harness\n'
	@printf '    Command: %s/%s\n\n' "$(BUILD_DIR)" "$(BINARY_NAME)"
	@$(BUILD_DIR)/$(BINARY_NAME) $(RUN_ARGS) || { \
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
	@bash scripts/verify-structure.sh

verify: verify-structure
	@printf '==> Running tests\n'
	@go test ./... || { \
		status=$$?; \
		printf '\n[fail] Tests failed with exit status %s.\n' "$$status"; \
		exit $$status; \
	}
	@printf '[ok] All checks passed\n'

# Advisory by default: prints the ranked hot spots without failing. Add
# -strict or -max-complexity=N when ratcheting a threshold.
complexity:
	@printf '==> Complexity report (source only, tests excluded)\n'
	@cd scripts/verify/complexity-report && go run . -root ../../.. $(COMPLEXITY_ARGS)
