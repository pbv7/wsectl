BINARY ?= wsectl
MODULE ?= github.com/pbv7/wsectl
BIN_DIR ?= dist
BIN ?= $(BIN_DIR)/$(BINARY)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS ?= -ldflags "-s -w -X $(MODULE)/internal/app.Version=$(VERSION) -X $(MODULE)/internal/app.Commit=$(COMMIT) -X $(MODULE)/internal/app.Date=$(DATE)"
COVERAGE_DIR ?= coverage
COVERAGE_OUT ?= $(COVERAGE_DIR)/coverage.out
COVERAGE_MIN ?= 70.0
GOLANGCI_LINT_CACHE ?= $(CURDIR)/.cache/golangci-lint
NODE_MODULES_LOCK ?= node_modules/.package-lock.json

.PHONY: check ci tidy tidy-check deps test race vet fmt fmt-check docs docs-check lint lint-md lint-shell lint-workflows lint-all vuln build build-linux build-darwin build-windows build-all run install version coverage coverage-check coverage-html live-test live-probe live-output-matrix release release-check snapshot release-snapshot clean

check: fmt-check tidy-check vet docs-check test

ci: check race lint-all vuln release-check

tidy:
	go mod tidy

tidy-check:
	go mod tidy -diff

deps:
	go mod download
	go mod tidy
	npm ci

fmt:
	gofmt -w cmd internal

fmt-check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal && exit 1)

vet:
	go vet ./...

test:
	go test ./...

race:
	go test -race ./...

build:
	mkdir -p $(dir $(BIN))
	go build $(LDFLAGS) -o $(BIN) ./cmd/wsectl

build-linux:
	mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-linux-amd64 ./cmd/wsectl
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-linux-arm64 ./cmd/wsectl

build-darwin:
	mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-darwin-amd64 ./cmd/wsectl
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-darwin-arm64 ./cmd/wsectl

build-windows:
	mkdir -p $(BIN_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-windows-amd64.exe ./cmd/wsectl
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY)-windows-arm64.exe ./cmd/wsectl

build-all: build-linux build-darwin build-windows

run: build
	$(BIN) $(ARGS)

install:
	go install $(LDFLAGS) ./cmd/wsectl

version:
	@echo "VERSION=$(VERSION)"
	@echo "COMMIT=$(COMMIT)"
	@echo "DATE=$(DATE)"

docs:
	go run ./cmd/wsectl docs generate --out docs/command-reference.md

docs-check:
	@tmp="$$(mktemp)"; trap 'rm -f "$$tmp"' EXIT; go run ./cmd/wsectl docs generate --out "$$tmp"; diff -u docs/command-reference.md "$$tmp"

lint:
	GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) golangci-lint run

$(NODE_MODULES_LOCK): package.json package-lock.json
	npm ci

lint-md: $(NODE_MODULES_LOCK)
	npm run --silent lint:md

lint-workflows:
	go tool actionlint .github/workflows/*.yml

lint-shell:
	@if command -v shellcheck >/dev/null 2>&1; then \
		shellcheck scripts/*.sh; \
	else \
		echo "shellcheck not installed; skipping local check (brew install shellcheck / apt-get install shellcheck)" >&2; \
	fi

lint-all: lint lint-workflows lint-md lint-shell

vuln:
	govulncheck ./...
	npm audit --audit-level=high

coverage:
	mkdir -p $(COVERAGE_DIR)
	go test -covermode=atomic -coverprofile=$(COVERAGE_OUT) ./...
	go tool cover -func=$(COVERAGE_OUT)

coverage-check: coverage
	@total="$$(go tool cover -func=$(COVERAGE_OUT) | awk '/^total:/ {print $$3}' | tr -d '%')"; \
	awk -v t="$$total" -v m="$(COVERAGE_MIN)" 'BEGIN { if (t+0 < m+0) { printf "coverage %s%% below threshold %s%%\n", t, m; exit 1 } printf "coverage %s%% meets threshold %s%%\n", t, m }'

coverage-html: coverage
	go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_DIR)/index.html

live-test:
	go test ./internal/worksection -run LiveSmoke -count=1

live-probe: build
	WSECTL=$(BIN) bash scripts/live-probe.sh

live-output-matrix: build
	WSECTL=$(BIN) bash scripts/live-output-matrix.sh

release-check:
	@if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then echo "Skipping goreleaser check: not a git repository"; elif ! git remote get-url origin >/dev/null 2>&1; then echo "Skipping goreleaser check: no origin remote configured"; else goreleaser check; fi

snapshot:
	goreleaser release --snapshot --clean

release-snapshot: snapshot

release:
	goreleaser release --clean

clean:
	rm -rf dist $(COVERAGE_DIR) .cache .tmp node_modules
