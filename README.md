# wsectl

Unofficial command-line client for Worksection.

`wsectl` is a Go CLI for read-only Worksection access. It is built for humans at a terminal and for scripts or coding agents that need stable JSON, predictable exit codes, explicit account profiles, and safe credential handling.

This project is not affiliated with, endorsed by, or supported by Worksection.

## Status

The MVP is read-only by design. Commands that would change Worksection data are recognized and blocked with a clear error. The code is structured so write commands can be added later behind explicit safety decisions.

## Start Here For Agents

Run the guide compiled into the installed binary before constructing commands:

```bash
wsectl help agent --full
```

For machine-readable discovery:

```bash
wsectl help agent --full --json
wsectl commands --json
wsectl api actions --json
wsectl api schema get_projects --json
wsectl doctor --json
```

The help JSON includes `guide_format_version`. `commands --json` reports explicit category, Worksection actions, output modes, authentication requirements, read-only status, examples, and agent notes.
`api schema ACTION --json` reports the static action contract, including parameters, response shape, known fields, conditional `extra` fields, count path, OAuth scopes, and compatibility notes.

## Install

From source:

```bash
go install github.com/pbv7/wsectl/cmd/wsectl@latest
```

For local development:

```bash
git clone https://github.com/pbv7/wsectl
cd wsectl
go run ./cmd/wsectl
```

`go run` is useful for help and development checks. For keyring-backed OAuth login, prefer a stable binary built with `make build` or an installed `wsectl`; temporary `go run` binaries can confuse OS keychain access control on macOS.

## Quick Start

Create a profile, log in with OAuth, then use JSON output for automation:

```bash
wsectl profiles add default --account-url https://company.worksection.com --auth-type oauth2
# Set WSECTL_CLIENT_ID and WSECTL_CLIENT_SECRET in your shell first.
wsectl auth login --client-id "$WSECTL_CLIENT_ID"
wsectl doctor --api
wsectl projects list --json
wsectl tasks search --query "invoice" --json
```

`wsectl auth login` starts a temporary local HTTPS callback server, opens the browser, validates OAuth state, exchanges the returned code, and stores tokens in the selected secret store.

For a remote shell or headless session, keep the callback server active and open the URL manually:

```bash
# Set WSECTL_CLIENT_ID and WSECTL_CLIENT_SECRET in your shell first.
wsectl auth login --no-browser --client-id "$WSECTL_CLIENT_ID"
```

For CI, use environment credentials:

```bash
WSECTL_ACCOUNT_URL=https://company.worksection.com \
WSECTL_ACCESS_TOKEN=... \
wsectl me --json
```

## Persistent Desktop Setup

For regular use on a personal workstation, prefer a config profile plus OS keychain secrets instead of exporting tokens every time.
See [Configuration](docs/configuration.md) for the full desktop, admin-token, encrypted-file, and CI workflows.

```bash
wsectl profiles add default \
  --account-url https://company.worksection.com \
  --auth-type oauth2

wsectl profiles use default

# Set WSECTL_CLIENT_ID and WSECTL_CLIENT_SECRET in your shell first.
wsectl auth login --client-id "$WSECTL_CLIENT_ID"

wsectl doctor --api
wsectl me --json
```

`profiles add` writes non-secret account settings to `config.toml`. When `--secret-ref` is omitted, the profile uses `keyring:wsectl/PROFILE`, so `auth login` stores OAuth tokens and client credentials in the OS keychain. After this setup, normal commands do not need `WSECTL_ACCESS_TOKEN`, `WSECTL_ACCOUNT_URL`, or other credential environment variables.

Default config locations:

- macOS/Linux: `~/.config/wsectl/config.toml` unless `$XDG_CONFIG_HOME` is set.
- Windows: `%AppData%\wsectl\config.toml`.

Use separate profiles for separate accounts or auth modes:

```bash
wsectl profiles add client-a --account-url https://client-a.worksection.com --auth-type oauth2
wsectl profiles add client-b --account-url https://client-b.worksection.com --auth-type oauth2
wsectl --profile client-a projects list --json
wsectl profiles use client-b
```

For an admin API key, keep a separate profile:

```bash
wsectl profiles add admin \
  --account-url https://company.worksection.com \
  --auth-type admin_token \
  --secret-ref keyring:wsectl/admin

# Set WSECTL_ADMIN_TOKEN in your shell first.
wsectl --profile admin auth login
wsectl --profile admin doctor --api
```

For a portable encrypted secret file instead of the OS keychain:

```bash
export WSECTL_SECRET_PASSPHRASE="use-a-password-manager-value"

wsectl profiles add portable \
  --account-url https://company.worksection.com \
  --auth-type oauth2 \
  --secret-ref encrypted-file:$HOME/.config/wsectl/secrets/portable.json

# Set WSECTL_CLIENT_ID and WSECTL_CLIENT_SECRET in your shell first.
wsectl --profile portable auth login --client-id "$WSECTL_CLIENT_ID"
```

The encrypted file is useless without `WSECTL_SECRET_PASSPHRASE`; do not store that passphrase next to the encrypted file.

## Common Reads

```bash
wsectl me --json
wsectl users list --json
wsectl projects list --status active --extra text,options,users --json
wsectl projects get 123 --extra text,users --json
wsectl tasks all --extra text,files --json --out /tmp/tasks.json
wsectl tasks list --project 123 --status active --extra text,comments --json
wsectl tasks search --query "invoice" --json
wsectl comments list 456 --extra files --json
wsectl costs total --project 123 --start 01.05.2026 --end 31.05.2026 --json
wsectl files download 789 --out ./attachment.bin
```

Use the lower-level API escape hatch when a read-only action is not wrapped by a first-class command:

```bash
wsectl api call get_users_schedule \
  --param datestart=01.05.2026 \
  --param dateend=31.05.2026 \
  --json
```

## Profiles

Profiles keep account and credential references separate:

```bash
wsectl profiles add default --account-url https://company.worksection.com --auth-type oauth2
wsectl profiles add admin --account-url https://company.worksection.com --auth-type admin_token
wsectl profiles use default
wsectl profiles list --json
wsectl profiles show default --json
```

Use `--profile NAME` when a command must target a specific account:

```bash
wsectl --profile default projects list --json
```

Config management commands:

```bash
wsectl profiles list --json
wsectl profiles show default --json
wsectl profiles use default
wsectl auth status --json
wsectl auth refresh
wsectl auth logout
wsectl profiles remove old-profile
```

Use `--config PATH` or `WSECTL_CONFIG` for a non-default config file. Use `WSECTL_PROFILE` or `--profile NAME` to override `current_profile` without editing config.

## Output For Scripts

Prefer JSON or NDJSON:

```bash
wsectl users list --json
wsectl tasks all --extra text,files --ndjson
wsectl projects list --json --jq '.data[] | {id, name, status}'
wsectl tasks all --json --fields id,name,status --out /tmp/tasks.json
wsectl tasks search --schema --json
```

Do not parse table output in scripts. Table output is for humans and may change to improve readability. JSON envelope fields are intended to stay stable:

```json
{
  "status": "ok",
  "data": [],
  "meta": {
    "action": "get_tasks",
    "profile": "default",
    "account_url": "https://company.worksection.com",
    "contract_version": "2026-06-06.1",
    "response_shape": "array",
    "count": 0,
    "truncated": false,
    "warnings": []
  }
}
```

Always check `meta.truncated` and `meta.warnings` for large responses.

`--schema --json` on first-class read commands returns the same static action contract without loading config, opening the keychain, or calling Worksection. These contracts are advisory and agent-readable; they are not full JSON Schema and do not guarantee that every field appears in every account.

## Security Model

Secrets are accessed through a `SecretStore` abstraction:

- `keyring`: default OS keychain backend.
- `env`: read-only backend for CI and containers.
- `encrypted-file`: explicit portable fallback protected by `WSECTL_SECRET_PASSPHRASE` with versioned Argon2id/AES-GCM payloads.
- `plaintext`: explicit opt-in only.

Commands do not print tokens by default. Use environment credentials for ephemeral automation and OS keychain storage for interactive use.

## Worksection API Limits

Worksection documents these API constraints:

- 1 request per second.
- GET URL length limit of 8 kB.
- Some endpoints can return at most 10,000 records.
- Some long text fields can be shortened by the server.

`wsectl` uses Worksection's documented/examples-compatible `POST` requests with API parameters in the query string and enforces client-side rate limiting by default, but it cannot remove server-side limits. Very long filters can still hit request URL limits.

## API Compatibility Notes

Official Worksection web docs, official Postman collections, and live API behavior can disagree. `wsectl` treats Postman/live behavior as authoritative for wire behavior and documents known normalizations.

- First-class commands may expose human-readable values while sending Worksection's raw API value. For example, `wsectl projects list --status archived` sends `filter=archive`.
- Low-level `api call` uses raw API parameter names and values. For archived projects, use `wsectl api call get_projects --param filter=archive --json`.
- `wsectl tasks search --query TEXT` sends an escaped Worksection `filter`, not an undocumented `search` parameter.
- `wsectl costs ... --timer true` sends `is_timer=true`.
- `wsectl files images` filters client-side after `get_files`.
- Live probes showed query-parameter POST works, JSON request bodies work for selected OAuth API reads, and `application/x-www-form-urlencoded` API bodies return `invalid JSON`. This build does not use form-encoded API bodies.

## Runtime Help

`wsectl`, `wsectl --help`, and `wsectl help` print the same deterministic start screen. Subcommand help remains command-specific.

```bash
wsectl
wsectl help agent --full
wsectl doctor
wsectl doctor --api --json
wsectl help completion
wsectl help output
wsectl commands --json
wsectl api actions --json
```

## Shell Completion

Completion scripts are generated by Cobra from the current command tree, so they stay aligned with new commands, flags, and enum completions when the binary is updated.

```bash
wsectl completion bash
wsectl completion zsh
wsectl completion fish
wsectl completion powershell
```

Examples:

```bash
wsectl completion zsh > "${fpath[1]}/_wsectl"
wsectl completion fish > ~/.config/fish/completions/wsectl.fish
wsectl completion powershell > wsectl.ps1
```

## Repository Docs

- [Manual](docs/manual.md)
- [Agent usage](docs/agent-usage.md)
- [Auth](docs/auth.md)
- [Configuration](docs/configuration.md)
- [Doctor](docs/doctor.md)
- [Shell completion](docs/completion.md)
- [Output contracts](docs/output-contracts.md)
- [Recipes](docs/recipes.md)
- [API coverage](docs/api-coverage.md)
- [Security](docs/security.md)
- [Release](docs/release.md)
- [Generated command reference](docs/command-reference.md)
- [Contributing](CONTRIBUTING.md)

## Development

```bash
make check
make ci
make coverage
make coverage-html
make build
make docs
make clean
```

Optional live smoke tests are disabled by default:

```bash
WSECTL_LIVE_TESTS=1 \
WSECTL_TEST_ACCOUNT_URL=https://company.worksection.com \
WSECTL_TEST_ACCESS_TOKEN=... \
go test ./internal/worksection -run LiveSmoke
```
