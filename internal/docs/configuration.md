# Configuration

`wsectl` keeps non-secret account settings in a TOML config file. Tokens and API keys are stored separately through secret references.

## Default Locations

- Linux and macOS: `$XDG_CONFIG_HOME/wsectl/config.toml` or `~/.config/wsectl/config.toml`
- Windows: `%AppData%\wsectl\config.toml`

Override the path with:

```bash
wsectl --config /path/to/config.toml profiles list
```

or:

```bash
WSECTL_CONFIG=/path/to/config.toml wsectl profiles list
```

## Example

```toml
current_profile = "default"

[defaults]
output = "auto"
rate_limit = "1/s"
timeout = "30s"

[profiles.default]
account_url = "https://company.worksection.com"
auth_type = "oauth2"
secret_ref = "keyring:wsectl/default"

[profiles.admin]
account_url = "https://company.worksection.com"
auth_type = "admin_token"
secret_ref = "keyring:wsectl/admin"
```

## Precedence

Configuration is resolved in this order:

1. Command flags
2. Environment variables
3. Profile config
4. Defaults from config
5. Built-in defaults

Example:

```bash
wsectl --profile admin --output json webhooks list
```

overrides both `current_profile` and the configured default output.

## Profile Commands

```bash
wsectl profiles add default --account-url https://company.worksection.com --auth-type oauth2
wsectl profiles add admin --account-url https://company.worksection.com --auth-type admin_token
wsectl profiles list --json
wsectl profiles show default --json
wsectl profiles use admin
wsectl profiles remove old-profile
```

## Working Flows

### Set Credential Variables

`auth login` reads only the secret variable for the active profile type: OAuth2 profiles use `WSECTL_CLIENT_SECRET`, while admin-token profiles use `WSECTL_ADMIN_TOKEN`. This keeps secret values out of command arguments and works across common shells.

sh, bash, and zsh:

```bash
export WSECTL_CLIENT_ID="your-client-id"
export WSECTL_CLIENT_SECRET="your-client-secret"
export WSECTL_ADMIN_TOKEN="your-admin-token"
```

PowerShell:

```powershell
$env:WSECTL_CLIENT_ID = "your-client-id"
$env:WSECTL_CLIENT_SECRET = "your-client-secret"
$env:WSECTL_ADMIN_TOKEN = "your-admin-token"
```

CMD:

```cmd
set WSECTL_CLIENT_ID=your-client-id
set WSECTL_CLIENT_SECRET=your-client-secret
set WSECTL_ADMIN_TOKEN=your-admin-token
```

### Desktop OAuth With OS Keychain

This is the recommended setup for persistent local use:

```bash
wsectl profiles add default \
  --account-url https://company.worksection.com \
  --auth-type oauth2

wsectl profiles use default

wsectl auth login --client-id "$WSECTL_CLIENT_ID"

wsectl doctor --api
```

When `--secret-ref` is omitted, `profiles add` uses `keyring:wsectl/default`. The TOML file stores the account URL, auth type, and secret reference. OAuth tokens, refresh tokens, and client secrets are stored in the OS keychain.

### Multiple Accounts

Use one profile per Worksection account:

```bash
wsectl profiles add agency --account-url https://agency.worksection.com --auth-type oauth2
wsectl profiles add client --account-url https://client.worksection.com --auth-type oauth2
wsectl --profile agency auth login --client-id "$WSECTL_CLIENT_ID"
wsectl --profile client auth login --client-id "$WSECTL_CLIENT_ID"
```

If each profile uses a different OAuth app, update `WSECTL_CLIENT_ID` and `WSECTL_CLIENT_SECRET` before each OAuth2 `auth login`. Admin-token profiles ignore OAuth client variables.

Run one command against a profile:

```bash
wsectl --profile agency projects list --json
```

Change the default profile:

```bash
wsectl profiles use client
```

Temporarily override the active profile without editing config:

```bash
WSECTL_PROFILE=agency wsectl tasks search --query invoice --json
```

### Admin Token Profile

Keep admin-token access separate from normal OAuth access:

```bash
wsectl profiles add admin \
  --account-url https://company.worksection.com \
  --auth-type admin_token \
  --secret-ref keyring:wsectl/admin

wsectl --profile admin auth login
wsectl --profile admin doctor --api
```

### Portable Encrypted File

Use `encrypted-file:` when OS keychain access is unavailable or you need a portable secret file:

```bash
export WSECTL_SECRET_PASSPHRASE="use-a-password-manager-value"

wsectl profiles add portable \
  --account-url https://company.worksection.com \
  --auth-type oauth2 \
  --secret-ref encrypted-file:$HOME/.config/wsectl/secrets/portable.json

wsectl --profile portable auth login --client-id "$WSECTL_CLIENT_ID"
```

The encrypted file is protected by `WSECTL_SECRET_PASSPHRASE`. Keep that passphrase in a password manager or another protected system, not next to the encrypted file.

### Environment-Only CI

Use environment credentials for CI, containers, and short-lived automation:

```bash
WSECTL_ACCOUNT_URL=https://company.worksection.com \
WSECTL_ACCESS_TOKEN=... \
wsectl projects list --json
```

You can also create a profile that points at the read-only env secret backend:

```bash
wsectl profiles add ci \
  --account-url https://company.worksection.com \
  --auth-type oauth2 \
  --secret-ref env:
```

`auth login` cannot write to `env:`. Set credential variables before running read commands.

## Managing Existing Config

Inspect the active configuration:

```bash
wsectl profiles list --json
wsectl profiles show default --json
wsectl auth status --json
wsectl doctor
```

Change defaults by editing `config.toml` or by using profile commands. `wsectl` writes config files with owner-only permissions and keeps profile order deterministic.

Rotate credentials:

```bash
wsectl auth logout
wsectl auth login --client-id "$WSECTL_CLIENT_ID"
```

Remove an unused profile:

```bash
wsectl profiles remove old-profile
```

## Environment Variables

Common variables:

```text
WSECTL_PROFILE
WSECTL_CONFIG
WSECTL_ACCOUNT_URL
WSECTL_OUTPUT
WSECTL_TIMEOUT
WSECTL_RATE_LIMIT
WSECTL_DEBUG
```

Credential variables are documented in [Auth](auth.md).

## Validate Setup

```bash
wsectl doctor
wsectl doctor --json
```

Doctor validates the config path, parse result, permissions, active profile, account URL, secret reference, credential presence, OAuth expiry, timeout, and rate limit without contacting Worksection. Use `wsectl doctor --api` for a live authenticated check.

Profile account URLs must be HTTPS Worksection URLs. `secret_ref` must use one of `keyring:`, `env:`, `encrypted-file:`, or `plaintext:`; non-env stores require a target name or path.
