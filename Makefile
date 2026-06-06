BIN ?= dist/wsectl
COVERAGE_DIR ?= coverage
COVERAGE_OUT ?= $(COVERAGE_DIR)/coverage.out
GOLANGCI_LINT_CACHE ?= $(CURDIR)/.cache/golangci-lint

.PHONY: check ci tidy tidy-check test race vet fmt fmt-check docs docs-check lint vuln build install coverage coverage-html live-test release-check snapshot clean

check: fmt-check tidy-check vet docs-check test

ci: check race lint vuln release-check

tidy:
	go mod tidy

tidy-check:
	go mod tidy -diff

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
	go build -o $(BIN) ./cmd/wsectl

install:
	go install ./cmd/wsectl

docs:
	go run ./cmd/wsectl docs generate --out docs/command-reference.md

docs-check:
	@tmp="$$(mktemp)"; trap 'rm -f "$$tmp"' EXIT; go run ./cmd/wsectl docs generate --out "$$tmp"; diff -u docs/command-reference.md "$$tmp"

lint:
	GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) golangci-lint run

vuln:
	govulncheck ./...

coverage:
	mkdir -p $(COVERAGE_DIR)
	go test -covermode=atomic -coverprofile=$(COVERAGE_OUT) ./...
	go tool cover -func=$(COVERAGE_OUT)

coverage-html: coverage
	go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_DIR)/index.html

live-test:
	go test ./internal/worksection -run LiveSmoke -count=1

release-check:
	@if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then echo "Skipping goreleaser check: not a git repository"; elif ! git remote get-url origin >/dev/null 2>&1; then echo "Skipping goreleaser check: no origin remote configured"; else goreleaser check; fi

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf dist $(COVERAGE_DIR) .cache .tmp
