# Contributing

`wsectl` is pre-release. Prefer small, focused changes that keep the read-only CLI correct, scriptable, and safe.

## Branch Protection And PR Flow

`main` is protected by a GitHub repository ruleset
([`.github/rulesets/main-branch.json`](.github/rulesets/main-branch.json)):

- All changes land via pull request. Direct `git push origin main` is rejected.
- PRs cannot merge until `tests`, `quality`, `race`, and `dependency-review` pass.
- Merges require linear history (squash-merge produces this automatically).
- Force-push and deletion of `main` are blocked.

Release tags (`v*`) are protected by a separate ruleset
([`.github/rulesets/release-tags.json`](.github/rulesets/release-tags.json)) that
blocks force-push and deletion.

### Normal Flow

```bash
git switch -c my-change
# ...edit, commit...
git push -u origin my-change
gh pr create --fill
# wait for checks, then:
gh pr merge --squash --delete-branch
```

### Admin Bypass

Repository admins can bypass the ruleset when merging a PR — useful for urgent
fixes when a check is broken for reasons unrelated to the change. Use sparingly:

```bash
gh pr merge --squash --admin
```

`--admin` requires admin role and uses the "Merge without waiting for
requirements to be met" path; it does not allow direct pushes to `main`
(bypass mode is `pull_request`, not `always`).

For tag operations during release recovery (e.g., re-tagging after a botched
push), the admin bypass on the tag ruleset is `always`, so admins can
`git push --force-with-lease origin vX.Y.Z` directly when needed. See
[`docs/release.md`](docs/release.md#recovery).

### Updating The Rulesets

The JSON files in `.github/rulesets/` are the source of truth. Apply changes
with:

```bash
scripts/apply-rulesets.sh                # apply to pbv7/wsectl
DRY_RUN=1 scripts/apply-rulesets.sh      # preview without calling the API
```

The script is idempotent (matched by `.name`) and also enables repo-level
`delete_branch_on_merge`. Requires `gh` (authenticated) and `jq`.

If you change the ruleset via the GitHub web UI directly, re-export the JSON
from the API and commit it so the file does not drift:

```bash
gh api repos/pbv7/wsectl/rulesets > /tmp/rulesets.json
# ...extract the relevant ruleset and update .github/rulesets/*.json...
```

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
make lint-shell
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

`make lint-shell` runs `shellcheck` against every `scripts/*.sh` file. Install it locally with `brew install shellcheck` on macOS or
`apt-get install shellcheck` on Debian/Ubuntu (Windows contributors typically run the bash toolchain under WSL and install it the same way as
Ubuntu). When shellcheck is not installed, the target prints a warning and exits cleanly so contributors without it can still run `make ci`. CI
installs shellcheck on demand and enforces the check there, so any regression lands on the PR rather than blocking local development. New shell
scripts must be clean against shellcheck's default warning set; suppress findings with inline `# shellcheck disable=SCxxxx` only with a
justification comment.

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

The following live checks assert **exit codes and envelope/output shapes** against a live account:

- `make live-probe` (`scripts/live-probe.sh`)
- `make live-output-matrix` (`scripts/live-output-matrix.sh`)
- the `LiveSmoke` test above

None of them run in CI (they need a live account, which untrusted pull requests cannot use), so they will not catch drift for you. When you change an
action contract, parameter validation, an output shape, or exit-code behavior, update their expectations in the same change — alongside bumping
`ContractVersion` and running `make docs` for the static contract and generated docs.
