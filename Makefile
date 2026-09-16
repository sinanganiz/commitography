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

# Toolchain versions the gates build with (ADR-0049 clause 6). CI reads them
# from here through `make toolchain-versions`, so the pin has one source.
#
# go.mod declares an older go directive than the semantics the tree already
# relies on; WP-0005 clause 10 raises the declared version. Until then the
# gates build with the pinned toolchain, which GOTOOLCHAIN selects regardless
# of which go is on PATH.
GO_VERSION   := 1.27.0
NODE_VERSION := 24.18.1
export GOTOOLCHAIN := go$(GO_VERSION)

# Verification tools, pinned. `go run` fetches them into the module cache;
# neither appears in go.mod, so neither is a direct dependency of the module
# (ADR-0049 clause 2).
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
GOVULNCHECK   := go run golang.org/x/vuln/cmd/govulncheck@v1.8.0

# Unit tests and checkers are separate gate entries, so the checker package is
# not also run as part of the unit test target.
GO_UNIT_PACKAGES := $(shell go list ./... | grep -vE '/internal/checks(/|$$)')

.PHONY: build web test fixtures lint clean docker-image docker-smoke perfcheck \
	gate-fast gate-full toolchain-versions build-go lint-go test-go checks \
	typecheck-web test-web vulncheck-go vulncheck-web fixture-determinism

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

# Measures analysis time, server responsiveness, cancellation latency, retained
# memory, startup and Docker overhead against generated repositories. Set
# COMMITOGRAPHY_PERF_REPO to add a real repository.
perfcheck:
	go test -tags perfcheck -count=1 -v -timeout 60m ./internal/perfcheck

# Full-gate checkers need inputs their own targets prepare, so they are left
# to those targets.
test: fixtures
	go test -skip '$(FULL_CHECKERS)' ./...

fixtures:
	sh testdata/build-fixtures.sh

# testdata/fixtures holds generated repositories whose .go files are line-noise
# by design. The go tool never loads a testdata directory, so no check here
# reaches them.
lint: lint-go

# ---------------------------------------------------------------- gates --
#
# ADR-0057 defines two gates with duration budgets. CI runs these same targets,
# so a gate has the same contents locally and there.
#
# Every implemented check belongs to exactly one gate (ADR-0057 clause 5).
# Exceeding a budget is never resolved by removing a check (clause 3).
#
#   fast (5 minutes)   build, format, vet, configuration rules, unit tests,
#                      record integrity, taxonomy integrity, decision
#                      reference, dependency allow list, goroutine ownership,
#                      frontend type check and unit tests
#   full (15 minutes)  everything in fast, plus vulnerability scanning and
#                      fixture determinism (on every supported platform)
#
# Checks ADR-0057 assigns to a gate whose subject does not exist yet — golden
# comparison, invariants, family contract, namespace violation, goroutine leak,
# leak scan, determinism, incremental equivalence, identity projection, mode
# capability matrix, model-free equivalence, performance budgets, subprocess
# count, cross-compilation, bundle integrity — are added by the package that
# creates each subject. WP-0004 clause 8 adds the golden comparisons.
FAST_CHECKS := build-go lint-go test-go checks typecheck-web test-web
FULL_CHECKS := vulncheck-go vulncheck-web fixture-determinism

# Checkers that belong to the full gate and are therefore skipped by `checks`.
FULL_CHECKERS := ^TestFixtureDeterminism$$

# Every gate prints the number of checks run, passed, failed and skipped
# (ADR-0064 clause 4). Go tests and frontend tests count one per test; every
# other step is one check. All steps run even after one fails, so the summary
# is complete; the gate then fails if any step did.
GATE_DIR    := out/gate
GATESUMMARY := go run ./internal/checks/gatesummary

define run-gate
	@rm -rf $(GATE_DIR) && mkdir -p $(GATE_DIR)
	@status=0; \
	for step in $(1); do \
		if $(MAKE) --no-print-directory $$step; then result=pass; else result=fail; status=1; fi; \
		$(GATESUMMARY) step $(GATE_DIR) $$step $$result || status=1; \
	done; \
	$(GATESUMMARY) report $(GATE_DIR) $(2) || status=1; \
	exit $$status
endef

# Fixture generation is gate setup, not a check: every fixture-dependent test
# needs it, and a gate establishes its checks' preconditions first (ADR-0064
# clause 1). A generation failure stops the gate before any check runs.
gate-fast: fixtures
	$(call run-gate,$(FAST_CHECKS),fast)

gate-full: fixtures
	$(call run-gate,$(FAST_CHECKS) $(FULL_CHECKS),full)

# Read by CI to pin its toolchain to the versions above.
toolchain-versions:
	@echo "go=$(GO_VERSION)"
	@echo "node=$(NODE_VERSION)"

build-go:
	go build ./...

# Formatting, vet and every rule ADR-0056 table 1 puts in linter configuration.
lint-go:
	$(GOLANGCI_LINT) run ./...

test-go:
	go test -json $(GO_UNIT_PACKAGES) | $(GATESUMMARY) gotest $(GATE_DIR) test-go

# The checkers read tracked files, so their result depends on the index rather
# than on package sources alone; -count=1 keeps the test cache out of it.
checks:
	go test -json -count=1 -skip '$(FULL_CHECKERS)' ./internal/checks/... | $(GATESUMMARY) gotest $(GATE_DIR) checks

# ADR-0019 clause 1. Generates the fixture set twice, outside the default
# fixture root, and compares both generations with each other and with
# testdata/fixture-hashes.txt. The runs sit under a directory named testdata
# so that the go tool does not load the generated .go files.
FIXTURE_RUNS := out/testdata/fixture-determinism

fixture-determinism:
	rm -rf $(FIXTURE_RUNS)
	sh testdata/build-fixtures.sh $(FIXTURE_RUNS)/first >/dev/null
	sh testdata/build-fixtures.sh $(FIXTURE_RUNS)/second >/dev/null
	COMMITOGRAPHY_FIXTURES_FIRST='$(CURDIR)/$(FIXTURE_RUNS)/first' \
	COMMITOGRAPHY_FIXTURES_SECOND='$(CURDIR)/$(FIXTURE_RUNS)/second' \
	go test -json -count=1 -run '$(FULL_CHECKERS)' ./internal/checks \
		| $(GATESUMMARY) gotest $(GATE_DIR) fixture-determinism

typecheck-web:
	cd web && npm ci && npm run typecheck

test-web:
	rm -f $(GATE_DIR)/vitest-report.json
	cd web && npm ci && npm test -- --reporter=default --reporter=json \
		--outputFile.json=../$(GATE_DIR)/vitest-report.json
	$(GATESUMMARY) vitest $(GATE_DIR) test-web $(GATE_DIR)/vitest-report.json

vulncheck-go:
	$(GOVULNCHECK) ./...

# Runtime dependencies only. The development tree carries advisories that can
# be resolved only by changing web/package.json, which is outside WP-0003;
# widening this to the whole tree belongs with the package that updates the
# frontend manifest (WP-0046).
vulncheck-web:
	cd web && npm audit --package-lock-only --omit=dev

clean:
	rm -rf $(BINARY) $(BINARY).exe out web/dist dist
