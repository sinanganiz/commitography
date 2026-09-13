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

.PHONY: build web test fixtures lint clean docker-image docker-smoke

build: web
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/commitography

web:
	cd web && npm ci && npm run build

# The Dockerfile copies a prebuilt Linux binary from its build context, which
# goreleaser supplies for releases. This target supplies the same context for
# a local image, built for the architecture of the Docker daemon in use. It
# embeds the committed frontend bundle; run `make web` first after UI changes.
DOCKER_IMAGE ?= commitography:local
DOCKER_ARCH  ?= $(shell docker version --format '{{.Server.Arch}}' 2>/dev/null)

docker-image:
	@test -n "$(DOCKER_ARCH)" || (echo "docker-image: the Docker daemon is not reachable" && exit 1)
	rm -rf dist/docker
	mkdir -p dist/docker
	CGO_ENABLED=0 GOOS=linux GOARCH=$(DOCKER_ARCH) go build -trimpath -ldflags "$(LDFLAGS)" -o dist/docker/commitography ./cmd/commitography
	cp Dockerfile dist/docker/Dockerfile
	docker build -t $(DOCKER_IMAGE) dist/docker

# Runs the image against the fixtures in CLI and server mode. The tests build
# their own image, or test COMMITOGRAPHY_IMAGE when it is set, and need Docker
# and `make fixtures`.
docker-smoke:
	go test -tags dockersmoke -count=1 -v ./internal/dockersmoke

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
