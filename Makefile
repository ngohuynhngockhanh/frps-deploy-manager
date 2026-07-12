# frps-deploy-manager — build & tooling
#
# Go is bootstrapped locally into ~/go-sdk (no system install / no sudo).
# Run `make bootstrap-go` once if the toolchain is missing.

GO       ?= $(HOME)/go-sdk/bin/go
GOVER    ?= go1.26.5
BIN      := bin/frps-deploy-manager
PKG      := ./cmd/frps-deploy-manager

# Version stamped into the binary
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -X main.version=$(VERSION)

export GOFLAGS ?=
export CGO_ENABLED = 0

.PHONY: all build run test tidy fmt vet clean bootstrap-go build-frps build-fpmultiuser

all: build

## bootstrap-go: download the official Go toolchain into ~/go-sdk
bootstrap-go:
	@if [ ! -x "$(GO)" ]; then \
		echo "Downloading $(GOVER) into ~/go-sdk ..."; \
		tmp=$$(mktemp -d); \
		curl -fsSL -o $$tmp/go.tgz "https://go.dev/dl/$(GOVER).linux-amd64.tar.gz"; \
		rm -rf $(HOME)/go-sdk && mkdir -p $(HOME)/go-sdk; \
		tar -C $(HOME)/go-sdk --strip-components=1 -xzf $$tmp/go.tgz; \
		rm -rf $$tmp; \
	fi
	@$(GO) version

## build: compile the manager binary
build:
	@mkdir -p bin
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) $(PKG)
	@echo "built $(BIN) ($(VERSION))"

## run: build + run on 127.0.0.1:2050
run: build
	./$(BIN)

## test: unit tests
test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

clean:
	rm -rf bin dist

## build-frps: cross-compile frps (linux/amd64) from ~/ksp-frp into bin/
build-frps:
	@mkdir -p bin
	cd $(HOME)/ksp-frp && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		$(GO) build -trimpath -o $(CURDIR)/bin/frps ./cmd/frps
	@echo "built bin/frps"

## build-fpmultiuser: cross-compile fp-multiuser (linux/amd64) from third_party/
build-fpmultiuser:
	@mkdir -p bin
	cd third_party/fp-multiuser && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		$(GO) build -trimpath -o $(CURDIR)/bin/fp-multiuser ./cmd/fp-multiuser
	@echo "built bin/fp-multiuser"
