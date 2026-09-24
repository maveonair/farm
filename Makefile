DIST_DIR := dist
BINARY := $(DIST_DIR)/farm
VERSION ?= $(shell version=$$(git describe --tags --always --dirty 2>/dev/null) && printf '%s' "$${version#v}" || printf dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || printf none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GO_LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.DEFAULT_GOAL := build

.PHONY: build web-install web-build web-test test vet check clean docs-serve docs-build

docs-serve:
	uv run --locked --only-group docs mkdocs serve

docs-build:
	uv run --locked --only-group docs mkdocs build --strict

web-install:
	cd web && pnpm install --frozen-lockfile

web-build: web-install
	cd web && pnpm run build

build: web-build
	mkdir -p $(DIST_DIR)
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o $(BINARY) ./cmd/farm

web-test: web-install
	cd web && pnpm run test:unit --run

test: web-test
	go test ./...

vet:
	go vet ./...

check: test vet

clean:
	rm -rf $(DIST_DIR)
