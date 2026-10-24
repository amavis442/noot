# noot — Makefile for Windows (cmd.exe, PowerShell, Git Bash) and Linux.
#
#   make              build for this machine → bin/noot or bin/noot.exe
#   make test         go vet + go test
#   make dist         cross-compile release binaries into dist/
#   make help         list all targets
#
# Windows: GNU make via `choco install make`, `scoop install make` or `winget install GnuWin32.Make`.

# ---------------------------------------------------------------- platform --
ifeq ($(OS),Windows_NT)
  # Always use cmd.exe on Windows, also when make is started from Git Bash,
  # so the recipes below behave the same everywhere.
  SHELL       := cmd.exe
  .SHELLFLAGS := /c
  NULL        := NUL
  RMDIR        = if exist $(subst /,\,$(1)) rmdir /s /q $(subst /,\,$(1))
  NATIVE       = $(subst /,\,$(1))
else
  NULL        := /dev/null
  RMDIR        = rm -rf $(1)
  NATIVE       = ./$(1)
endif

# ------------------------------------------------------------------ config --
BINARY  := noot
PKG     := ./cmd/noot
MODULE  := github.com/amavis442/noot
BIN_DIR := bin
DIST    := dist

GO      ?= go
EXE     := $(shell $(GO) env GOEXE)
VERSION ?= $(or $(shell git describe --tags --always --dirty 2>$(NULL)),dev)
LDFLAGS := -s -w -X $(MODULE)/internal/cli.Version=$(VERSION)
GOFLAGS_BUILD := -trimpath -ldflags "$(LDFLAGS)"

# The SQLite driver is pure Go/Wasm, so no C compiler is needed anywhere and
# every binary is static.
export CGO_ENABLED := 0

# OS/arch pairs for `make dist`.
PLATFORMS := linux/amd64 linux/arm64 windows/amd64 windows/arm64

# ----------------------------------------------------------------- targets --
.DEFAULT_GOAL := build
.PHONY: build build-linux build-windows dist install run test vet fmt tidy clean help _cross

build: # Build for this machine → bin/noot(.exe)
	$(GO) build $(GOFLAGS_BUILD) -o $(BIN_DIR)/$(BINARY)$(EXE) $(PKG)

build-linux: # Cross-compile bin/noot for linux/amd64
	@"$(MAKE)" --no-print-directory _cross GOOS=linux GOARCH=amd64 OUT=$(BIN_DIR)/$(BINARY)

build-windows: # Cross-compile bin/noot.exe for windows/amd64
	@"$(MAKE)" --no-print-directory _cross GOOS=windows GOARCH=amd64 OUT=$(BIN_DIR)/$(BINARY).exe

dist: # Release binaries for $(PLATFORMS) → dist/
	@$(foreach p,$(PLATFORMS),"$(MAKE)" --no-print-directory _cross \
		GOOS=$(word 1,$(subst /, ,$(p))) GOARCH=$(word 2,$(subst /, ,$(p))) \
		OUT=$(DIST)/$(BINARY)-$(subst /,-,$(p))$(if $(findstring windows,$(p)),.exe) &&) echo dist ready

# Internal: GOOS/GOARCH passed on the make command line end up in the recipe's
# environment on every shell (no `GOOS=x go build` syntax needed).
_cross:
	@echo building $(GOOS)/$(GOARCH): $(OUT)
	$(GO) build $(GOFLAGS_BUILD) -o $(OUT) $(PKG)

install: # go install with version info (→ $GOBIN or $GOPATH/bin)
	$(GO) install $(GOFLAGS_BUILD) $(PKG)

run: build # Build and run: make run ARGS="search sqlite"
	$(call NATIVE,$(BIN_DIR)/$(BINARY)$(EXE)) $(ARGS)

test: vet # go vet + go test
	$(GO) test ./...

vet: # go vet
	$(GO) vet ./...

fmt: # gofmt -w
	$(GO) fmt ./...

tidy: # go mod tidy
	$(GO) mod tidy

clean: # Remove bin/ and dist/
	$(call RMDIR,$(BIN_DIR))
	$(call RMDIR,$(DIST))

help: # Show this help
	@echo Targets:
	@echo   build          build for this machine: $(BIN_DIR)/$(BINARY)$(EXE)  [default]
	@echo   build-linux    cross-compile $(BIN_DIR)/$(BINARY) for linux/amd64
	@echo   build-windows  cross-compile $(BIN_DIR)/$(BINARY).exe for windows/amd64
	@echo   dist           release binaries in $(DIST)/ for: $(PLATFORMS)
	@echo   install        go install with version info
	@echo   run            build and run, e.g. make run ARGS="search sqlite"
	@echo   test           go vet + go test
	@echo   vet / fmt / tidy
	@echo   clean          remove $(BIN_DIR)/ and $(DIST)/
	@echo Version: $(VERSION)
