# Contributing

`wsectl` is pre-release. Prefer small, focused changes that keep the read-only CLI correct, scriptable, and safe.

## Local Checks

Run the fast gate before handing off changes:

```bash
make check
```

For broader verification:

```bash
make race
make lint
make lint-md
make lint-workflows
make vuln
make coverage
make release-check
```

`make coverage` may need loopback access because some tests use `httptest.Server`.

`make lint` intentionally lints production Go files only for now. Local Go 1.26.4 with golangci-lint 2.12.2 can fail while loading test packages with
`no go files to analyze`, even though `go test` and `go list` succeed. Tests are still compiled and run by `make check` and `make race`; re-enable
golangci-lint test-file linting after the toolchain issue is resolved.

`make lint-md` uses the lockfile-managed `markdownlint-cli2` npm dev dependency with the prose line limit from `.markdownlint.json`. The rule is
relaxed for code blocks, headings, and tables, but prose warnings are fixed instead of disabled. `make lint-workflows` runs the `actionlint` Go tool
declared in `go.mod` against GitHub Actions workflow files.

`make vuln` runs `govulncheck` for Go packages and `npm audit --audit-level=high` for the Markdown lint toolchain.

## Building From Source

`go install ./cmd/wsectl` and `go build ./cmd/wsectl` work out of the box. The set of credential backends in the
resulting binary depends on `CGO_ENABLED`:

| Build                              | Keyring backends compiled in                             |
| ---------------------------------- | -------------------------------------------------------- |
| Default (`CGO_ENABLED=1` on macOS) | Keychain (macOS), Secret Service, KWallet, Pass, WinCred |
| `CGO_ENABLED=0` on macOS           | Secret Service, KWallet, Pass, WinCred — **no Keychain** |
| Default on Linux / Windows         | Secret Service, KWallet, Pass, WinCred (none need cgo)   |

`encrypted-file`, `env`, and `plaintext` backends are pure-Go and always available.

Why: the macOS Keychain backend in `github.com/99designs/keyring` calls Apple's Security.framework through cgo
(`//go:build darwin && cgo`). Building wsectl on macOS with `CGO_ENABLED=0` silently drops it. See
[`docs/security.md`](docs/security.md#keyring-backend-availability-by-build) for the runtime matrix and
[`docs/adr/0001-cgo-and-keyring-backends.md`](docs/adr/0001-cgo-and-keyring-backends.md) for the decision history.

If you are testing release-pipeline changes that affect cgo or signing, `make release-snapshot` produces a `dist/`
tree matching the release format.

## Documentation

Command reference docs are generated from command metadata:

```bash
make docs
```

Use `make docs-check` or `make check` to verify checked-in command docs are current. Embedded runtime docs intentionally mirror selected files in
`docs/`; tests fail if those copies drift.

## Live Tests

Live tests are opt-in and must not run on untrusted pull requests:

```bash
WSECTL_LIVE_TESTS=1 \
WSECTL_TEST_ACCOUNT_URL=https://company.worksection.com \
WSECTL_TEST_ACCESS_TOKEN=... \
go test ./internal/worksection -run LiveSmoke -count=1
```

Keep live tests read-only.
