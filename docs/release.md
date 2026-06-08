# Release

Releases use GoReleaser triggered from Git tags. The workflow publishes a public GitHub release and updates the Homebrew tap in
the same run. Both operations use separate tokens — `GITHUB_TOKEN` (built into Actions) for the release, and `HOMEBREW_TAP_TOKEN`
(a fine-grained PAT) for the tap push.

## One-Time Setup

The release workflow publishes a Homebrew cask to `pbv7/homebrew-tap`. This requires a fine-grained PAT, configured once:

1. Generate a token at <https://github.com/settings/personal-access-tokens/new> scoped to **only**
   `pbv7/homebrew-tap` with `Contents: read+write`.
2. Add to `pbv7/wsectl` → Settings → Secrets and variables → Actions → New repository secret named
   `HOMEBREW_TAP_TOKEN`.

Without this secret, the release workflow fails at the `token-check` job before any artifacts are produced.

## Pre-Tag Checklist

Run locally on a clean working tree:

1. `make ci` — full local gates (check, race, lint-all, vuln, release-check)
2. `make coverage-check` — coverage gate against the configured minimum
3. `make release-snapshot` — local artifacts build cleanly
4. Inspect `dist/`:
   - Archives per OS/arch (`*.tar.gz`, `*.zip`)
   - `checksums.txt`
   - `*.sbom.json` per archive
   - Generated cask at `dist/homebrew/Casks/wsectl.rb`
5. Inspect generated release notes in the snapshot output: confirm grouped sections
   (Features, Bug Fixes, Performance, Refactoring, Dependencies, Others), no missing-or-duplicate entries
6. `WSECTL_HISTORY=0 make live-probe` against your test account: confirm end-to-end binary behavior on real data

Item 6 is the only check that exercises a real Worksection account. Skipping it means tagging blind on live behavior.
(`make ci` is fine locally because the developer machine has the toolchain. The release workflow uses dedicated Actions
for the same steps, since GitHub-hosted runners do not have those binaries preinstalled.)

## Tag And Push

Use annotated semver tags:

```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
gh run watch
```

The release workflow runs three jobs in sequence: `token-check` → `preflight` → `goreleaser`. Each step's failure
mode is handled in Recovery below.

## Post-Release Verification

On a clean machine if possible:

```bash
brew tap pbv7/tap
brew install wsectl
brew test wsectl
wsectl version    # prints "wsectl 0.1.0"
```

Also check:

- GitHub release at <https://github.com/pbv7/wsectl/releases> is public (not draft), with grouped changelog
  and all assets present
- `https://github.com/pbv7/homebrew-tap/blob/main/Casks/wsectl.rb` exists with the new version

## Recovery

Partial-publish is possible because the GitHub release publish and the tap push are not transactional:

- **Workflow fails at `token-check` or `preflight`**: no artifacts published. Fix the cause on `main`, then delete
  the tag and retag:

  ```bash
  git tag -d v0.1.0
  git push origin :refs/tags/v0.1.0
  # ...fix...
  git tag -a v0.1.0 -m "Release v0.1.0"
  git push origin v0.1.0
  ```

- **Binary release succeeds, tap push fails**: the GitHub release is already public. Do not delete it.
  Verify `HOMEBREW_TAP_TOKEN`, then re-run the release workflow on the same tag:

  ```bash
  gh workflow run release.yml -f tag=v0.1.0
  ```

  If GoReleaser reports asset conflicts during the retry, resolve them (the simplest path is to delete the
  conflicting assets from the GitHub release UI) before re-running. The tap push will then proceed.

- **Bad version tagged accidentally** (typo, wrong commit): if no one has installed yet, delete the GitHub
  release and tag, fix, retag. Once users have installed, treat the tag as published and ship the fix
  as `v0.1.1`.

## Notes On Naming

- GitHub repository: `pbv7/homebrew-tap`
- Homebrew tap name: `pbv7/tap` (Homebrew strips the `homebrew-` prefix automatically)

`brew install pbv7/tap/wsectl` and `brew install pbv7/homebrew-tap/wsectl` reach the same cask.
