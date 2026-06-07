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

Before tagging, also run one real file download probe against a safe task attachment:

```bash
wsectl files download FILE_ID --out /tmp/wsectl-probe.bin
```

The probe passes when the output file is non-empty, filename or content type looks correct, and no blocked-host error is returned.

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
