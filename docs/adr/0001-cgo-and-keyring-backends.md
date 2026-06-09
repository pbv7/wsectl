# ADR 0001: CGO And Keyring Backends

- Date: 2026-06-09
- Status: Accepted
- Supersedes: —

## Context

`wsectl` uses [`github.com/99designs/keyring`](https://github.com/99designs/keyring) for OS-native credential storage.
Backends in that library have different build constraints:

- macOS Keychain — `//go:build darwin && cgo`. Calls Apple's Security.framework through cgo.
- Windows Credential Manager — no cgo. Pure-Go Win32 syscalls.
- Linux Secret Service — no cgo. Pure-Go dbus client.
- Linux KWallet — no cgo. Pure-Go dbus client.
- Pass — no cgo. Shells out to the `pass` CLI.
- File, KeyCtl — intentionally disabled by `wsectl` (the File backend stores secrets in a passphrase-protected file the
  library treats as a fallback; we prefer the explicit `encrypted-file` backend instead, which gives stronger
  guarantees and clearer error messages).

The v0.1.0 release was built with `CGO_ENABLED=0` on a Linux runner. On macOS that compiled the Keychain backend out
of the binary entirely. The failure mode was confusing:

- `wsectl doctor` reported `[ok] secret_backend: keyring backend is supported`, because the check only constructs the
  store wrapper struct.
- `wsectl auth login` then failed with `secret store is not writable: Specified keyring backend not available`,
  because `keyring.Open` returns that error when none of the allowed backends matched a compiled-in backend.
- macOS never prompted for keychain access, because the binary never reached the Security.framework call.

`Security.framework` shows up in `otool -L` on the v0.1.0 binary even though cgo was disabled. That is a red herring:
`crypto/x509` links it on darwin for certificate verification.

## Decision

Release artifacts enable cgo for darwin builds only. Linux and Windows stay at `CGO_ENABLED=0`.

In `.goreleaser.yaml`:

```yaml
builds:
  - env:
      - CGO_ENABLED=0          # default for linux, windows
    overrides:
      - goos: darwin
        goarch: arm64
        env:
          - CGO_ENABLED=1
          - CC=clang -arch arm64
          - CXX=clang++ -arch arm64
      - goos: darwin
        goarch: amd64
        env:
          - CGO_ENABLED=1
          - CC=clang -arch x86_64
          - CXX=clang++ -arch x86_64
```

The release job in `.github/workflows/release.yml` runs on `macos-latest` so cgo darwin builds have a working
Apple toolchain. The preflight job stays on `ubuntu-latest`.

Source builds (`go install`, `go build`) follow the caller's `CGO_ENABLED`. On macOS the Go default is 1, so
contributors and `go install` users get the Keychain backend. Users who explicitly set `CGO_ENABLED=0` (Alpine/musl
containers, hardened environments) get a binary without the Keychain backend and must use `encrypted-file:` or
`env:`.

## Alternatives Considered

1. **Always-on cgo across every OS.** Rejected. Linux cgo binaries dynamically link glibc and break on musl
   distributions; Windows cgo requires a MinGW cross-toolchain; CI gets slower and artifacts get larger. No upside,
   since Linux and Windows backends do not need cgo.

2. **Drop `99designs/keyring`, shell out to platform CLIs** (`security`, `secret-tool`, `cmdkey`). Rejected for now.
   Adds a runtime dependency that may be absent in containers, exposes credentials through process arguments and
   audit logs, and forces us to re-implement uniform error semantics across three platforms.

3. **Replace Keychain with `encrypted-file` on macOS by default.** Rejected. Degrades the default UX (a passphrase
   prompt on every login) to work around a release-pipeline configuration. The fix belongs in the pipeline, not the
   UX.

4. **Keep `CGO_ENABLED=0` releases, document `encrypted-file` as the macOS default.** Rejected. Makes the most common
   platform the worst experience.

5. **Use `goreleaser-cross` Docker images for darwin cgo from a Linux runner.** Rejected. Requires Apple SDK
   redistribution (license-gray) and adds image-version drift on top of the existing toolchain pin. A native
   `macos-latest` runner is cleaner.

## Consequences

- The release job runs on `macos-latest`. macOS minutes bill 10× ubuntu on private repos and are free on public ones.
  Acceptable while the repo stays public; revisit if visibility changes.
- Cross-compiling `darwin/amd64` from an `arm64` runner with cgo depends on the x86_64 SDK being present in the
  GitHub-hosted image. GitHub publishes runner manifests; if Apple's universal SDK is ever dropped from the
  `macos-latest` image, the `darwin/amd64` build breaks. Mitigation: the preflight job runs `goreleaser check` and a
  snapshot build can be added there if this becomes flaky.
- A `go install` user on macOS who sets `CGO_ENABLED=0` gets a quietly degraded binary. `wsectl doctor` will report
  the missing backend at first login attempt with remediation pointing at `encrypted-file:PATH`.
- Apple Developer signing and notarization remain out of scope. See
  [`../security.md`](../security.md#binary-distribution-and-gatekeeper) "Binary Distribution And Gatekeeper".
- This ADR is the source of rationale. The runtime matrix (which backend works where) lives in
  [`../security.md`](../security.md#credential-storage) and is the file that needs editing if backend availability
  changes; this ADR only changes if the underlying decision is revisited.

## How To Verify

After cutting a new release, on a macOS machine:

```bash
brew upgrade pbv7/tap/wsectl
wsectl doctor                                                       # secret_backend ok
wsectl auth login --client-id "$ID" --client-secret "$SECRET"       # writes; prompts keychain once
wsectl doctor --api                                                 # credentials ok, api ok
```

The reliable indicator is that `auth login` succeeds and the macOS keychain prompts for write access. `otool -L` is
not a reliable check (see Context above).

## References

- `.goreleaser.yaml` — `builds[].overrides`
- `.github/workflows/release.yml` — runner choice
- `internal/auth/keyring_store.go` — allowed backends list
- `internal/doctor/doctor.go` — `secret_backend` / `credentials` checks
- `99designs/keyring` `keychain.go` — `//go:build darwin && cgo`
