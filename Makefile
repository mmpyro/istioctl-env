VERSION ?= 0.1.0
BINARY_NAME = istioctl-env
BUILD_DIR = build
LDFLAGS = -ldflags "-X main.Version=$(VERSION)"

.PHONY: build build-all test test-docker clean install docs-build docs-serve

## build: Build for current platform
build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/istioctl-env

## build-all: Cross-compile for all supported platforms
build-all:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/istioctl-env
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/istioctl-env
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/istioctl-env
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/istioctl-env

## test: Run unit tests
test:
	go test -v -race ./...

## test-docker: Run Docker integration tests
test-docker:
	./scripts/test-docker.sh

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

## install: Install to GOPATH/bin
install:
	go install $(LDFLAGS) ./cmd/istioctl-env

## help: Show this help
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

.venv-docs: docs/requirements.txt
	python3 -m venv .venv-docs
	.venv-docs/bin/pip install -q -r docs/requirements.txt
	touch .venv-docs

## docs-build: Build the docs site to ./site (strict)
docs-build: .venv-docs
	.venv-docs/bin/mkdocs build --strict

## docs-serve: Serve docs locally with live reload at http://127.0.0.1:8000/istioctl-env/
docs-serve: .venv-docs
	.venv-docs/bin/mkdocs serve
