# Manual

`wsectl` is an unofficial command-line client for Worksection. It focuses on read-only access, stable output, explicit profiles, and credential
handling that works across desktop, CI, and headless environments.

This project is not affiliated with Worksection.

## First Run

For agents and automation, start with the version-matched operating guide:

```bash
wsectl help agent --full
```

Create a profile for your Worksection account:

```bash
wsectl profiles add default --account-url https://company.worksection.com --auth-type oauth2
```

Start OAuth login:

```bash
# Set WSECTL_CLIENT_ID and WSECTL_CLIENT_SECRET in your shell first.
wsectl auth login --client-id "$WSECTL_CLIENT_ID"
```

Then run a read command:

```bash
wsectl doctor --api
wsectl projects list --json
```

`wsectl`, `wsectl --help`, and `wsectl help` show the same deterministic start screen.

## Authentication Modes

OAuth2 is the default. It stores tokens in the selected secret backend and refreshes them when needed.

Admin-token profiles are available for accounts that use Worksection's admin API signing:

```bash
wsectl profiles add admin --account-url https://company.worksection.com --auth-type admin_token
# Set WSECTL_ADMIN_TOKEN in your shell first.
wsectl --profile admin auth login
```

Environment credentials are useful for CI:

```bash
WSECTL_ACCOUNT_URL=https://company.worksection.com WSECTL_ACCESS_TOKEN=... wsectl me --json
```

## Profiles

Profiles select account URL, auth type, and secret reference:

```bash
wsectl profiles list --json
wsectl profiles show default --json
wsectl profiles use default
wsectl --profile admin webhooks list --json
```

For persistent PC usage, profiles are the normal configuration mechanism. `profiles add` writes non-secret metadata to `config.toml`; `auth login`
writes credentials to the profile's secret store. When `--secret-ref` is omitted, the profile uses `keyring:wsectl/PROFILE`. See
[Configuration](configuration.md) for the full desktop, admin-token, encrypted-file, and CI setup workflows.

Useful management commands:

```bash
wsectl profiles list --json
wsectl profiles show default --json
wsectl profiles use default
wsectl auth status --json
wsectl auth logout
wsectl profiles remove old-profile
```

Use `--profile NAME` for one command, `WSECTL_PROFILE=NAME` for a shell/session override, or `profiles use NAME` to change the default profile in
config.

Portable encrypted-file setup:

```bash
export WSECTL_SECRET_PASSPHRASE="use-a-password-manager-value"
wsectl profiles add portable --account-url https://company.worksection.com --auth-type oauth2 --secret-ref encrypted-file:$HOME/.config/wsectl/secrets/portable.json
wsectl --profile portable auth login --client-id "$WSECTL_CLIENT_ID"
```

## Reading Data

Common commands:

```bash
wsectl me --json
wsectl users list --json
wsectl users groups --json
wsectl projects list --status active --extra text,options,users --json
wsectl projects events --project 123 --period 7d --json
wsectl tasks all --status active --extra text,files --json --out /tmp/tasks.json
wsectl tasks list --project 123 --extra text,comments --json
wsectl tasks search --query "invoice" --json
wsectl comments list 456 --extra files --json
wsectl tags task list --type label --access public --json
wsectl costs list --project 123 --start 01.05.2026 --end 31.05.2026 --json
wsectl timers mine --json
wsectl files list --project 123 --json
wsectl webhooks list --json
```

## Low-Level API Calls

Use `api call` when the Worksection API exposes a read-only action that does not yet have a specific command:

```bash
wsectl api actions --json
wsectl api schema get_users_schedule --json
wsectl api call get_users_schedule --param datestart=01.05.2026 --param dateend=31.05.2026 --json
```

Known mutation actions are blocked even through `api call`.

`api schema ACTION --json` and command `--schema --json` expose static response contracts without calling Worksection:

```bash
wsectl tasks search --schema --json
```

These contracts include known fields, conditional `extra` fields, response shape, count path, and compatibility notes. They are advisory, not full
JSON Schema.

First-class commands normalize known Worksection API quirks. For example, `projects list --status archived` sends `filter=archive`,
`tasks search --query TEXT` sends a Worksection `filter`, `costs --timer` sends `is_timer`, and `files images` filters client-side. Raw `api call`
uses raw API parameter names and values.

## Diagnostics

Run local setup checks without making a network request:

```bash
wsectl doctor
wsectl doctor --json
```

Add `--api` to perform one authenticated read request:

```bash
wsectl doctor --api
```

See [Doctor](doctor.md) for checks and exit classifications.

## Files

List file metadata with JSON:

```bash
wsectl files list --project 123 --json
wsectl files task-attachments 456 --json
```

Download binary content with `--out`:

```bash
wsectl files download 789 --out ./attachment.bin
```

Downloads require `--out FILE` or explicit `--out -`.

## Output Modes

Use `--json` for stable structured output and `--ndjson` for large arrays:

```bash
wsectl tasks all --json --out /tmp/tasks.json
wsectl tasks all --ndjson --out /tmp/tasks.ndjson
```

Use `--fields` for simple projection and `--jq` for richer JSON filtering:

```bash
wsectl projects list --json --fields id,name,status
wsectl projects list --json --jq '.data[] | select(.status == "active") | {id, name}'
```

Table output is only for humans.

## Optional History

Command history is opt-in and stored as local JSONL metadata, never stdout logging:

```bash
WSECTL_HISTORY=1 wsectl projects list --json
wsectl history path --json
wsectl history list --json --limit 20
wsectl history clear --keep 1000
```

History records command metadata and non-secret parameters. It never stores full API responses, downloaded files, tokens, or authorization headers.
See [Configuration](configuration.md) for desktop, container, and Windows paths.

## Shell Completion

Generate completion scripts from the current binary:

```bash
wsectl completion bash
wsectl completion zsh
wsectl completion fish
wsectl completion powershell
```

The generated scripts come from Cobra's live command tree. After upgrading `wsectl`, regenerate the script for your shell to pick up new commands,
flags, and enum completions.

## Limits And Completeness

Worksection documents a 1 request/second API limit, an 8 kB GET URL limit, and a 10,000-record cap for some endpoints. `wsectl` sends API calls with
POST and rate-limits requests, but it keeps API parameters in the query string to match documented/live behavior, so very long filters can still hit
request URL limits. Large responses include metadata:

```json
{
  "meta": {
    "truncated": false,
    "warnings": []
  }
}
```

Use `--fail-on-truncated` when a partial result should fail automation.
