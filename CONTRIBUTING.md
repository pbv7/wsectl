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
