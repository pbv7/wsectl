# Release

Releases use GoReleaser and are intended to run from Git tags.

## Pre-Release Checks

Before tagging:

```bash
make check
make race
make lint
make vuln
make release-check
```

Run optional live smoke tests with real read-only credentials before the first public release.

## Snapshot Build

After the repository is initialized with Git, validate local release artifacts:

```bash
make snapshot
```

Snapshot artifacts are written to `dist/`, which is ignored by Git.

## Tag Release

Use semantic version tags:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The GitHub Actions release workflow runs GoReleaser for tags matching `v*`. Release archives include `README.md`, `LICENSE`, and `docs/`.
