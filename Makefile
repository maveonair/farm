DIST_DIR := dist
BINARY := $(DIST_DIR)/farm

.PHONY: build web-install web-build web-test test vet check clean

web-install:
	cd web && pnpm install --frozen-lockfile

web-build: web-install
	cd web && pnpm run build

build: web-build
	mkdir -p $(DIST_DIR)
	go build -o $(BINARY) ./cmd/farm

web-test: web-install
	cd web && pnpm run test:unit --run

test: web-test
	go test ./...

vet:
	go vet ./...

check: test vet

clean:
	rm -rf $(DIST_DIR)
