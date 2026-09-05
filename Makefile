MODULE := github.com/sinanganiz/commitography

# Windows will not execute a file without an extension, so the built binary
# needs one there and nowhere else.
BINARY := commitography
ifeq ($(OS),Windows_NT)
	BINARY := commitography.exe
endif

VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X '$(MODULE)/internal/version.Version=$(VERSION)' \
	-X '$(MODULE)/internal/version.Commit=$(COMMIT)' \
	-X '$(MODULE)/internal/version.BuildDate=$(BUILD_DATE)'

.PHONY: build web test fixtures lint clean

build: web
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/commitography

web:
	cd web && npm ci && npm run build

test:
	go test ./...

fixtures:
	sh testdata/build-fixtures.sh

# testdata/fixtures holds generated repositories whose .go files are line-noise
# by design, so formatting is checked against the source tree only.
lint:
	go vet ./...
	@test -z "$$(gofmt -l cmd internal | tee /dev/stderr)" || (echo "gofmt: files above are not formatted" && exit 1)

clean:
	rm -rf $(BINARY) $(BINARY).exe out web/dist dist
