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
make vuln
make coverage
make release-check
```

`make coverage` may need loopback access because some tests use `httptest.Server`.

`make lint` intentionally lints production Go files only for now. Local Go
1.26.4 with golangci-lint 2.12.2 can fail while loading test packages with
`no go files to analyze`, even though `go test` and `go list` succeed. Tests
are still compiled and run by `make check` and `make race`; re-enable
golangci-lint test-file linting after the toolchain issue is resolved.

## Documentation

Command reference docs are generated from command metadata:

```bash
make docs
```

Use `make docs-check` or `make check` to verify checked-in command docs are current. Embedded runtime docs intentionally mirror selected files in `docs/`; tests fail if those copies drift.

## Live Tests

Live tests are opt-in and must not run on untrusted pull requests:

```bash
WSECTL_LIVE_TESTS=1 \
WSECTL_TEST_ACCOUNT_URL=https://company.worksection.com \
WSECTL_TEST_ACCESS_TOKEN=... \
go test ./internal/worksection -run LiveSmoke -count=1
```

Keep live tests read-only.
