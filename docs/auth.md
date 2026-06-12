# Auth

`wsectl` supports OAuth2, admin-token profiles, environment credentials, and explicit file-backed secret stores. OAuth2 is the default.

Worksection OAuth redirect URIs must use HTTPS. For local login, `wsectl` starts a temporary HTTPS callback server on a loopback address and generates
a self-signed localhost certificate unless certificate files are supplied.

## Browser OAuth Login

```bash
wsectl profiles add default --account-url https://company.worksection.com --auth-type oauth2
wsectl auth login --client-id "$WSECTL_CLIENT_ID"
```

For OAuth2 profiles, set `WSECTL_CLIENT_ID` and `WSECTL_CLIENT_SECRET` in your shell first. See [Configuration](configuration.md) for sh/bash/zsh,
PowerShell, and CMD examples.

For persistent desktop usage, pass client credentials once and let `wsectl` store them in the selected secret backend:

```bash
make build

wsectl profiles add default \
  --account-url https://company.worksection.com \
  --auth-type oauth2

wsectl profiles use default

./dist/wsectl auth login --client-id "$WSECTL_CLIENT_ID"
```

The profile is saved in `config.toml`; tokens, refresh tokens, and client credentials are stored through the profile's `secret_ref`. The default
`secret_ref` is `keyring:wsectl/default`, which uses the OS keychain.

For keyring-backed login, prefer a stable binary such as `./dist/wsectl` or an installed `wsectl`. Repeated `go run ./cmd/wsectl ...` invocations use
temporary binaries and can confuse OS keychain access control, especially on macOS.

Flow:

1. Generate an OAuth `state`.
2. Start `https://localhost:33443/callback` by default.
3. Open the authorization URL in the default browser.
4. Validate the returned `state`.
5. Exchange the returned code for access and refresh tokens.
6. Store tokens in the selected secret store.

The callback host must be `localhost`, `127.0.0.1`, or `::1`. Port `0` is rejected because Worksection redirect URIs must be registered. Invalid-state
or unrelated callback requests are rejected in the browser but do not stop the login attempt.

Before opening the browser, `wsectl` checks that the selected secret backend is writable. OAuth token exchange and refresh requests use
timeout-bearing HTTP clients so network failures do not hang indefinitely.

Use a different callback binding when needed:

```bash
wsectl auth login \
  --client-id "$WSECTL_CLIENT_ID" \
  --callback-host localhost \
  --callback-port 4443 \
  --login-timeout 10m
```

Avoid ports already used by local services. On macOS, port `5000` is commonly occupied by Control Center/AirPlay Receiver; if the browser shows
`ERR_SSL_PROTOCOL_ERROR`, register and use another HTTPS callback port such as `33443`.

If the browser warns about the generated self-signed certificate, continue only for the expected localhost callback URL. `wsectl` never installs
certificate trust automatically.

Use your own certificate and key:

```bash
wsectl auth login \
  --client-id "$WSECTL_CLIENT_ID" \
  --callback-cert ./localhost.crt \
  --callback-key ./localhost.key
```

## No-Browser OAuth Login

Use `--no-browser` on remote shells or when browser launching is unavailable. The callback server still runs; you open the printed URL yourself.

```bash
wsectl auth login --no-browser --client-id "$WSECTL_CLIENT_ID"
```

## OAuth Scopes

By default, `auth login` requests the read scopes needed by the read-only command surface:

```text
projects_read tasks_read costs_read tags_read comments_read files_read users_read contacts_read
```

Request a custom least-privilege scope set by repeating `--scope`:

```bash
wsectl auth login \
  --client-id "$WSECTL_CLIENT_ID" \
  --scope projects_read \
  --scope tasks_read
```

Administrative/webhook access is not requested by default. Use an admin-token profile or explicitly configured administrative OAuth scope if your
Worksection app requires it.

## Authorization Code Login

If you already have an authorization code:

```bash
wsectl auth login \
  --client-id "$WSECTL_CLIENT_ID" \
  --code "$CODE"
```

The client secret is read from `WSECTL_CLIENT_SECRET`, or from `--client-secret-stdin` when stdin is safer for your launcher.

`--manual-code` prints an authorization URL and exits without waiting. It is useful when the callback cannot be reached from the browser environment.
The follow-up command expects the client secret to come from `WSECTL_CLIENT_SECRET` or `--client-secret-stdin`; do not paste client secrets into shell
history.

## Manual Token Storage

For controlled migration or test setup, tokens can be stored directly:

```bash
wsectl auth login --access-token "$WSECTL_ACCESS_TOKEN" --refresh-token "$WSECTL_REFRESH_TOKEN"
```

Do not use this path in scripts that might echo shell history or logs. Prefer normal OAuth login, environment credentials for CI, or a controlled
one-time migration session.

For an admin-token profile:

```bash
wsectl profiles add admin \
  --account-url https://company.worksection.com \
  --auth-type admin_token \
  --secret-ref keyring:wsectl/admin

wsectl --profile admin auth login
```

Only `admin_token` profiles read `WSECTL_ADMIN_TOKEN` during `auth login`; OAuth2 profiles ignore it. It is read from the environment when
`--admin-token` and `--admin-token-stdin` are omitted.

## Token Refresh

Refresh explicitly:

```bash
wsectl auth refresh
```

Read commands refresh OAuth access tokens automatically in two ways:

- **Proactive**: when the stored expiry is within five minutes, the token is refreshed before the request.
- **Reactive**: when the API rejects a request with HTTP 401 and the profile carries refresh material (a refresh token plus client ID and secret),
  the token is refreshed once and the request is replayed. At most one reactive refresh happens per command invocation; a second rejection is a
  normal authentication failure (exit 3). This covers tokens whose expiry was never recorded — including environment credentials.

After a successful refresh, the new tokens are written back to the profile's secret store. If that write fails, the command still completes using
the refreshed token and prints a warning on stderr (suppressed by `--quiet`). Environment credentials are read-only by design and skip the
write-back silently. `wsectl auth refresh` remains strict: an explicit refresh that cannot persist its result fails.

## Status And Logout

```bash
wsectl auth status --json
wsectl auth logout
```

`logout` deletes the selected profile's stored secret. It does not revoke tokens server-side.

To rotate stored credentials:

```bash
wsectl auth logout
wsectl auth login --client-id "$WSECTL_CLIENT_ID"
```

## Environment Credentials

Environment credentials are read-only and useful for CI:

```bash
WSECTL_ACCOUNT_URL=https://company.worksection.com \
WSECTL_ACCESS_TOKEN=... \
wsectl me --json
```

Supported variables include:

```text
WSECTL_ACCESS_TOKEN
WSECTL_REFRESH_TOKEN
WSECTL_ADMIN_TOKEN
WSECTL_CLIENT_ID
WSECTL_CLIENT_SECRET
WSECTL_ACCOUNT_URL
```

## Secret Stores

Four backends are supported: `keyring`, `env`, `encrypted-file`, `plaintext`. The full table — including which
backend is available on which OS and in which build variant — lives in
[`security.md`](security.md#credential-storage). Quick orientation:

- `keyring` is the default. Uses the OS-native credential store (Keychain, Credential Manager, Secret Service,
  KWallet, Pass). Availability depends on the build; macOS Keychain in particular requires a cgo-enabled build.
- `env` is read-only. Intended for CI, containers, and ephemeral automation.
- `encrypted-file` is the portable fallback. Protected by `WSECTL_SECRET_PASSPHRASE` with versioned Argon2id/AES-GCM
  payloads. Legacy payloads are read and rewritten in the new format on the next save. Use this when `keyring` is
  unavailable or you need a profile that travels between machines.
- `plaintext` is explicit opt-in only. Human-mode login prints a warning to stderr before writing plaintext secrets.

No command prints tokens by default.

`auth login` verifies that the selected secret store is writable before starting browser OAuth. Environment
credentials are read-only, so use `keyring`, `encrypted-file`, or `plaintext` for interactive login.

If doctor reports `secret store is not writable: Specified keyring backend not available`, the build does not
include a working OS keyring backend for this platform (most commonly: a `CGO_ENABLED=0` build on macOS, which
drops the Keychain backend). Either reinstall the release artifact, rebuild from source with `CGO_ENABLED=1`, or
migrate the profile to `encrypted-file:PATH` and log in again. See
[`security.md`](security.md#keyring-backend-availability-by-build) for the full matrix.

Portable encrypted-file profile:

```bash
export WSECTL_SECRET_PASSPHRASE="use-a-password-manager-value"

wsectl profiles add portable \
  --account-url https://company.worksection.com \
  --auth-type oauth2 \
  --secret-ref encrypted-file:$HOME/.config/wsectl/secrets/portable.json

wsectl --profile portable auth login --client-id "$WSECTL_CLIENT_ID"
```

The encrypted file can be backed up or moved, but it cannot be read without `WSECTL_SECRET_PASSPHRASE`.

## Stdin Secret Input

For shells or launchers where environment variables are not appropriate, read a secret from one line on stdin:

sh, bash, and zsh:

```bash
printf '%s\n' "$WSECTL_CLIENT_SECRET" | wsectl auth login --client-id "$WSECTL_CLIENT_ID" --client-secret-stdin
printf '%s\n' "$WSECTL_ADMIN_TOKEN" | wsectl --profile admin auth login --admin-token-stdin
```

PowerShell:

```powershell
$env:WSECTL_CLIENT_SECRET | wsectl auth login --client-id $env:WSECTL_CLIENT_ID --client-secret-stdin
$env:WSECTL_ADMIN_TOKEN | wsectl --profile admin auth login --admin-token-stdin
```

CMD:

```cmd
echo %WSECTL_CLIENT_SECRET%| wsectl auth login --client-id %WSECTL_CLIENT_ID% --client-secret-stdin
echo %WSECTL_ADMIN_TOKEN%| wsectl --profile admin auth login --admin-token-stdin
```

`--client-secret-stdin` is valid for OAuth2 profiles and is mutually exclusive with `--client-secret`. `--admin-token-stdin` is valid for admin-token
profiles and is mutually exclusive with `--admin-token`.

## Diagnose Authentication

```bash
wsectl doctor
wsectl doctor --api
wsectl doctor --api --json
```

Doctor reports credential presence and OAuth expiry without printing credential values. The live check performs one authenticated read request: `me`
for OAuth profiles and `get_users` for admin-token profiles.
