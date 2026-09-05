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

# -s -w strip the symbol table and DWARF. On Darwin that also removes the
# LC_UUID load command, and dyld refuses to start a Mach-O binary without one
# ("dyld: missing LC_UUID load command", Abort trap: 6). The size saving is not
# worth an unrunnable binary, so macOS builds keep their symbols.
TARGET_GOOS ?= $(shell go env GOOS)
STRIP_FLAGS := -s -w
ifeq ($(TARGET_GOOS),darwin)
	STRIP_FLAGS :=
endif

LDFLAGS := $(STRIP_FLAGS) \
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
