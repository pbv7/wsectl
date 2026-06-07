# Security

`wsectl` is designed to avoid accidental credential exposure and accidental data changes.

## Unofficial Tool

This project is not affiliated with Worksection. Users are responsible for granting appropriate OAuth scopes and protecting account credentials.

## Credential Storage

Secret backends:

- `keyring`: default. Uses OS-backed keychains or Pass through `github.com/99designs/keyring`; the library's File and KeyCtl backends are
  intentionally disabled.
- `env`: read-only. Intended for CI, containers, and ephemeral automation.
- `encrypted-file`: explicit opt-in. Requires `WSECTL_SECRET_PASSPHRASE` and writes versioned Argon2id/AES-GCM payloads.
- `plaintext`: explicit opt-in only. Intended for controlled testing, not normal use. Human-mode login warns on stderr before writing plaintext
  secrets.

Tokens are not stored in `config.toml`. Profiles contain a `secret_ref`, not the secret itself.

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
