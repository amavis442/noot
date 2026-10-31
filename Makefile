# noot — Makefile for Linux and Windows.
#
#   make              build for this machine → bin/noot or bin/noot.exe
#   make test         go vet + go test
#   make web          build the web frontend into web/dist (needs Node.js)
#   make dist         cross-compile release binaries into dist/
#   make help         list all targets
#
# Shell-agnostic on purpose: recipes only run `go`, `make` and `echo`, joined
# with `&&`. That works the same whether make hands them to sh (Linux, MSYS2,
# Git Bash) or to cmd.exe (native Windows make), so there is no OS detection
# and no SHELL override. File operations go through small Go helpers.

BINARY  := noot
PKG     := ./cmd/noot
MODULE  := github.com/amavis442/noot
BIN_DIR := bin
DIST    := dist

GO      ?= go
# npm through a Go helper, so it also runs from MSYS2 make on Windows.
NPM     ?= $(GO) run ./internal/tools/npm
EXE     := $(shell $(GO) env GOEXE)

# Without VERSION, noot reports the git commit Go embeds at build time
# (e.g. "1afc1bb" or "1afc1bb-dirty"). For releases: make dist VERSION=v0.2.0
VERSION ?=
LDFLAGS := -s -w$(if $(VERSION), -X $(MODULE)/internal/cli.Version=$(VERSION))
BUILD   := $(GO) build -trimpath -ldflags "$(LDFLAGS)"

# The SQLite driver is pure Go/Wasm: no C compiler needed, static binaries.
export CGO_ENABLED := 0

# OS/arch pairs for `make dist`.
PLATFORMS := linux/amd64 linux/arm64 windows/amd64 windows/arm64

.DEFAULT_GOAL := build
.PHONY: build build-linux build-windows dist install run test vet fmt tidy clean help _cross web web-test

build: # Build for this machine → bin/noot(.exe)
	$(BUILD) -o $(BIN_DIR)/$(BINARY)$(EXE) $(PKG)

build-linux: # Cross-compile bin/noot for linux/amd64
	"$(MAKE)" --no-print-directory _cross GOOS=linux GOARCH=amd64 OUT=$(BIN_DIR)/$(BINARY)

build-windows: # Cross-compile bin/noot.exe for windows/amd64
	"$(MAKE)" --no-print-directory _cross GOOS=windows GOARCH=amd64 OUT=$(BIN_DIR)/$(BINARY).exe

dist: web # Release binaries for $(PLATFORMS) → dist/, with the web frontend
	$(foreach p,$(PLATFORMS),"$(MAKE)" --no-print-directory _cross GOOS=$(word 1,$(subst /, ,$(p))) GOARCH=$(word 2,$(subst /, ,$(p))) OUT=$(DIST)/$(BINARY)-$(subst /,-,$(p))$(if $(findstring windows,$(p)),.exe) && ) echo dist ready

# Internal. GOOS/GOARCH given on make's command line are exported into the
# recipe's environment by make itself, so no `GOOS=x go build` shell syntax.
_cross:
	$(BUILD) -o $(OUT) $(PKG)

install: # go install → $GOBIN or $GOPATH/bin
	$(GO) install -trimpath -ldflags "$(LDFLAGS)" $(PKG)

run: # Run without installing: make run ARGS="search sqlite"
	$(GO) run $(PKG) $(ARGS)

test: vet # go vet + go test
	$(GO) test ./...

web: # Build the web frontend into web/dist, which noot embeds (needs Node.js)
	$(NPM) --prefix web ci && $(NPM) --prefix web run build

web-test: # Frontend tests (needs Node.js)
	$(NPM) --prefix web ci && $(NPM) --prefix web test

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

clean: # Remove bin/ and dist/
	$(GO) run ./internal/tools/rmdirs $(BIN_DIR) $(DIST)

help:
	@echo Targets:
	@echo   build          build for this machine: $(BIN_DIR)/$(BINARY)$(EXE) [default]
	@echo   build-linux    cross-compile $(BIN_DIR)/$(BINARY) for linux/amd64
	@echo   build-windows  cross-compile $(BIN_DIR)/$(BINARY).exe for windows/amd64
	@echo   dist           release binaries in $(DIST)/ for $(PLATFORMS)
	@echo   install        go install
	@echo   run            go run, e.g. make run ARGS=--version
	@echo   test           go vet + go test
	@echo   web            build the web frontend into web/dist [needs Node.js]
	@echo   web-test       frontend tests [needs Node.js]
	@echo   vet, fmt, tidy
	@echo   clean          remove $(BIN_DIR)/ and $(DIST)/
	@echo Set VERSION=v0.2.0 to stamp a release version.
