.PHONY: help build test mutation-test run clean install fmt lint release release-dry-run verify-structure verify complexity

BINARY_NAME := agent-harness
BUILD_DIR := ./build
RUN_ARGS ?=
MUTATION_TARGETS ?=
COMPLEXITY_ARGS ?=
RELEASE_ARGS ?=

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

# VERSION comes from git. The release tag on main is the only place a
# version lives, and git describe reports how far a build sits from it:
# v0.4.2 on the tag itself, v0.4.2-3-gabc1234 three commits later, and a
# -dirty suffix for uncommitted changes. Outside a git checkout it is dev.
VERSION := $(shell git describe --tags --match 'v[0-9]*' --always --dirty 2>/dev/null || echo dev)
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
	@printf '  make install        Install %s and the ah shortcut into GOPATH/GOBIN\n' "$(BINARY_NAME)"
	@printf '  make clean          Remove build artifacts\n'
	@printf '  make release        Tag origin/main as the next version and push it\n'
	@printf '  make release-dry-run  Show the next version and its commits, change nothing\n'
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

# A release is a tag on origin/main; release.yml builds and publishes it.
# RELEASE_ARGS passes options, e.g. RELEASE_ARGS="--bump minor".
release:
	@sh scripts/release/release.sh $(RELEASE_ARGS)

release-dry-run:
	@sh scripts/release/release.sh --dry-run $(RELEASE_ARGS)

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
	@go install -trimpath -ldflags "$(LDFLAGS)" "$(MAIN_PKG)" || { \
		status=$$?; \
		printf '\n[fail] Install failed with exit status %s.\n' "$$status"; \
		exit $$status; \
	}
	@printf '[ok] Installed %s %s\n' "$(BINARY_NAME)" "$(VERSION)"
ifneq ($(OS),Windows_NT)
	@bindir="$$(go env GOBIN)"; [ -n "$$bindir" ] || bindir="$$(go env GOPATH | cut -d: -f1)/bin"; \
	link="$$bindir/ah"; \
	if { [ ! -e "$$link" ] && [ ! -L "$$link" ]; } || [ "$$(readlink "$$link")" = "$(BINARY_NAME)" ]; then \
		ln -sfn "$(BINARY_NAME)" "$$link" && printf '[ok] Linked %s -> %s\n' "$$link" "$(BINARY_NAME)"; \
	else \
		printf '[skip] %s already exists and is not ours; ah was not linked\n' "$$link"; \
	fi
endif

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
