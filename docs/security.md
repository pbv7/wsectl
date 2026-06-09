# Security

`wsectl` is designed to avoid accidental credential exposure and accidental data changes.

## Unofficial Tool

This project is not affiliated with Worksection. Users are responsible for granting appropriate OAuth scopes and protecting account credentials.

## Credential Storage

`wsectl` supports four secret backends, selected per profile via `secret_ref`:

| Backend          | Description                                                                                                      | Availability                         |
| ---------------- | ---------------------------------------------------------------------------------------------------------------- | ------------------------------------ |
| `keyring`        | OS-native credential store (macOS Keychain, Windows Credential Manager, Secret Service, KWallet, Pass). Default. | Depends on build and OS — see below. |
| `env`            | Read-only environment variables. Intended for CI, containers, ephemeral automation.                              | All builds, all OS.                  |
| `encrypted-file` | Versioned Argon2id/AES-GCM file. Requires `WSECTL_SECRET_PASSPHRASE`.                                            | All builds, all OS.                  |
| `plaintext`      | Unencrypted JSON file. Explicit opt-in only; human-mode login warns on stderr before writing.                    | All builds, all OS. Not recommended. |

Tokens are never written to `config.toml`. Profiles hold a `secret_ref` only.

The 99designs/keyring `File` and `KeyCtl` backends are intentionally disabled. Use `encrypted-file` instead of the
library's File backend (stronger guarantees, clearer errors); KeyCtl secrets do not survive a reboot.

### Keyring Backend Availability By Build

The underlying OS backend behind `keyring:` depends on whether the binary was compiled with cgo, because the macOS
Keychain backend in `github.com/99designs/keyring` is gated by `//go:build darwin && cgo`. Other OS-native backends do
not need cgo.

| OS      | Native backend                  | Needs cgo | Release artifacts (Homebrew, GitHub release) | `go install` (default `CGO_ENABLED=1` on macOS) | `go install` with `CGO_ENABLED=0` |
| ------- | ------------------------------- | --------- | -------------------------------------------- | ----------------------------------------------- | --------------------------------- |
| macOS   | Keychain (Security.framework)   | yes       | available                                    | available                                       | **unavailable**                   |
| Linux   | Secret Service (dbus) / KWallet | no        | available                                    | available                                       | available                         |
| Linux   | Pass (CLI shellout)             | no        | available                                    | available                                       | available                         |
| Windows | Credential Manager              | no        | available                                    | available                                       | available                         |

If the `keyring` backend is not available for your build and OS, `wsectl auth login` fails with:

```text
secret store is not writable: Specified keyring backend not available
```

Migrate the profile to `encrypted-file:PATH` (portable, requires a passphrase) or `env:` (read-only, for CI), or
rebuild with `CGO_ENABLED=1`. See [`auth.md`](auth.md#secret-stores) for the migration steps and
[`adr/0001-cgo-and-keyring-backends.md`](adr/0001-cgo-and-keyring-backends.md) for the rationale and trade-offs
behind the build matrix.

## Token Output

Commands do not print tokens by default. Avoid `--debug` in shared logs. Any future command that exposes token material should require an explicit
dangerous flag.

Optional command history is disabled by default. When enabled, it writes local JSONL metadata only: command path, action, profile, output mode, exit
code, duration, counts, warnings, and non-secret parameters. It never records tokens, authorization headers, full API response bodies, or downloaded
file contents. Account URLs and queried IDs or filters can still reveal work context, so treat the history file as a local forensic surface and keep
it protected. History write failures do not change command exit codes; run `wsectl doctor` when history is enabled to verify the configured path is
writable. History does not auto-trim during command execution; use `wsectl history clear --keep N` to compact it manually. History appends and
compaction use a short-lived `.lock` file beside the history file to avoid concurrent rewrite races. Stale lock files older than 10 minutes are
removed automatically to recover from interrupted processes.

Downloads forward bearer credentials only to HTTPS URLs whose host matches the configured Worksection account host after normalization. Cross-host or
insecure file URLs are blocked with structured error details instead of being retried unauthenticated.

## OAuth Callback

Browser login uses a temporary local HTTPS server. The server:

- Generates or loads TLS credentials.
- Uses a random OAuth `state`.
- Validates the returned `state`.
- Listens only on loopback addresses.
- Rejects unrelated invalid-state requests without ending the login attempt.
- Exits after receiving a valid callback or when the login timeout/context is canceled.

Self-signed localhost certificates are generated for convenience. Use `--callback-cert` and `--callback-key` if your environment requires a
pre-trusted certificate.

## Read-Only Safety

The MVP blocks known mutation actions locally. This protects both first-class commands and `wsectl api call`.

## Binary Distribution And Gatekeeper

Released binaries are built by GoReleaser in GitHub Actions and are **not** Apple-signed or notarized. Two consequences:

- The Homebrew cask runs a postflight hook on macOS that strips the `com.apple.quarantine` attribute from the installed
  `wsectl` binary. Without this hook, the first invocation of `wsectl` would be blocked by Gatekeeper with a
  "developer cannot be verified" prompt. The hook is a no-op on Linux.
- `brew install pbv7/tap/wsectl` therefore trades one layer of macOS defense for installation ergonomics. The chain of
  trust is the tap itself: a compromise of `pbv7/homebrew-tap` or the release pipeline could ship a binary that runs
  without a Gatekeeper warning.

If you want Gatekeeper enforcement, use `go install` instead of Homebrew:

```bash
go install github.com/pbv7/wsectl/cmd/wsectl@latest
```

This builds from source on your machine; Gatekeeper does not apply.

To verify a downloaded archive against the release checksums:

```bash
shasum -a 256 wsectl_<version>_<os>_<arch>.tar.gz
# compare with the matching line in checksums.txt from the GitHub release
```

Apple Developer Program signing and notarization are out of scope for the current release cadence; revisit when project
audience justifies the setup cost.

## CI Recommendations

Use environment credentials and short-lived CI secrets:

```bash
WSECTL_ACCOUNT_URL=https://company.worksection.com WSECTL_ACCESS_TOKEN=... wsectl projects list --json
```

Do not write plaintext secrets into repository files or build logs.

For containers, prefer an explicit mounted history path or keep history disabled:

```bash
WSECTL_HISTORY=1 WSECTL_HISTORY_FILE=/state/history.jsonl wsectl doctor --api --json
```
