# Security

`wsectl` is designed to avoid accidental credential exposure and accidental data changes.

## Unofficial Tool

This project is not affiliated with Worksection. Users are responsible for granting appropriate OAuth scopes and protecting account credentials.

## Credential Storage

Secret backends:

- `keyring`: default. Uses the OS keychain or credential manager through `github.com/99designs/keyring`.
- `env`: read-only. Intended for CI, containers, and ephemeral automation.
- `encrypted-file`: explicit opt-in. Requires `WSECTL_SECRET_PASSPHRASE` and writes versioned Argon2id/AES-GCM payloads.
- `plaintext`: explicit opt-in only. Intended for controlled testing, not normal use.

Tokens are not stored in `config.toml`. Profiles contain a `secret_ref`, not the secret itself.

## Token Output

Commands do not print tokens by default. Avoid `--debug` in shared logs. Any future command that exposes token material should require an explicit dangerous flag.

## OAuth Callback

Browser login uses a temporary local HTTPS server. The server:

- Generates or loads TLS credentials.
- Uses a random OAuth `state`.
- Validates the returned `state`.
- Listens only on loopback addresses.
- Rejects unrelated invalid-state requests without ending the login attempt.
- Exits after receiving a valid callback or when the login timeout/context is canceled.

Self-signed localhost certificates are generated for convenience. Use `--callback-cert` and `--callback-key` if your environment requires a pre-trusted certificate.

## Read-Only Safety

The MVP blocks known mutation actions locally. This protects both first-class commands and `wsectl api call`.

## CI Recommendations

Use environment credentials and short-lived CI secrets:

```bash
WSECTL_ACCOUNT_URL=https://company.worksection.com WSECTL_ACCESS_TOKEN=... wsectl projects list --json
```

Do not write plaintext secrets into repository files or build logs.
