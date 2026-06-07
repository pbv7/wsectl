# Release

Releases use GoReleaser and are intended to run from Git tags.

## Pre-Release Checks

Before tagging:

```bash
make check
make race
make lint-all
make vuln
make release-check
```

`make lint-all` runs Go linting, Markdown linting, and GitHub Actions workflow linting. Markdownlint is managed through `package-lock.json`, and
actionlint is managed through Go's `tool` directive. Markdown prose uses the configured line-length rule while code blocks, headings, and tables are
exempt from that rule.

Run optional live smoke tests with real read-only credentials before the first public release.

`make coverage-check` is a POSIX-shell Makefile target with `COVERAGE_MIN ?= 70.0`. Keep it as a visibility gate until coverage is intentionally
raised above the threshold; Windows release validation should use the direct Go test commands from CI.

Before tagging, run the end-to-end live probe against a real read-only account:

```bash
make live-probe
```

The probe uses whatever auth your normal `wsectl` commands use, so a profile previously set up with `wsectl auth login`
is sufficient. For one-shot runs without a profile, export `WSECTL_ACCOUNT_URL` plus `WSECTL_ACCESS_TOKEN`
(or `WSECTL_ADMIN_TOKEN`) before running.

`make live-probe` runs `scripts/live-probe.sh`, which exercises the binary end-to-end: doctor, identity, projects, tasks, comments,
file listing, file download, output formats (`--json`, `--ndjson`, `--table`, `--fields`, `--limit`, `--schema`), low-level
`api call`, and the exit-code contract for negative cases. The script self-bootstraps IDs from `projects list` and `tasks list`;
override with `WSECTL_PROBE_PROJECT`, `WSECTL_PROBE_TASK`, or `WSECTL_PROBE_FILE` if you want to pin specific resources.
Set `WSECTL=./dist/wsectl` (or any built binary path) to verify release artifacts instead of `go run`.

The probe specifically verifies the same-host download policy: a successful `files download` confirms bearer credentials
reached the configured Worksection account without leaking cross-host.

## Snapshot Build

After the repository is initialized with Git, validate local release artifacts:

```bash
make release-snapshot
```

Snapshot artifacts are written to `dist/`, which is ignored by Git.

## Tag Release

Use semantic version tags:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The GitHub Actions release workflow runs GoReleaser for tags matching `v*`. Release archives include `README.md`, `LICENSE`, and `docs/`.
