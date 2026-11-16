# REM Go — production Makefile
# Requires Go 1.26+.  Run `make` for the default (all) target.

BINARY     := rem
CMD        := ./cmd/rem
GOPATH     ?= $(shell go env GOPATH)
INSTALL_DIR := $(GOPATH)/bin
DIST       := dist

LDFLAGS    := -s -w
BUILD_FLAGS := -trimpath -ldflags="$(LDFLAGS)"

.PHONY: all build test smoke install cross lint clean

## Default ─────────────────────────────────────────────────────────────────────
all: build test lint

## Build the binary for the current platform
build:
	go build $(BUILD_FLAGS) -o $(BINARY) $(CMD)

## Run the full test suite with race detector
test:
	go test -count=1 -race ./...

## Smoke-test the built binary against a live target.
## Override the target with REM_LIVE_TARGET=<domain> (default: linear.app).
smoke: build
	REM_LIVE_TARGET=$${REM_LIVE_TARGET:-linear.app} bash smoke.sh ./$(BINARY)

## Install the binary into $(GOPATH)/bin/
install: build
	install -m 0755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "installed → $(INSTALL_DIR)/$(BINARY)"

## Cross-compile release binaries into dist/
cross:
	mkdir -p $(DIST)
	GOOS=linux  GOARCH=amd64 go build $(BUILD_FLAGS) -o $(DIST)/$(BINARY)-linux-amd64   $(CMD)
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o $(DIST)/$(BINARY)-darwin-arm64  $(CMD)
	@echo "cross-compiled binaries written to $(DIST)/"

## Static analysis
lint:
	go vet ./...

## Remove compiled artifacts
clean:
	rm -f $(BINARY)
	rm -rf $(DIST)
