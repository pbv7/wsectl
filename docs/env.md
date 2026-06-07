# Environment Variables

## Runtime Selection

```text
WSECTL_PROFILE
WSECTL_CONFIG
WSECTL_ACCOUNT_URL
WSECTL_OUTPUT
WSECTL_TIMEOUT
WSECTL_RATE_LIMIT
WSECTL_DEBUG
```

## Optional History

```text
WSECTL_HISTORY
WSECTL_HISTORY_FILE
WSECTL_HISTORY_PARAMS
```

`WSECTL_HISTORY=1` enables the opt-in local JSONL history file. Use `WSECTL_HISTORY_FILE` to choose a persistent path, especially in containers.
`WSECTL_HISTORY_FILE` alone does not enable history. `WSECTL_HISTORY_PARAMS` can be `none`, `safe`, or `all`.

## Credentials

```text
WSECTL_ACCESS_TOKEN
WSECTL_REFRESH_TOKEN
WSECTL_ADMIN_TOKEN
WSECTL_CLIENT_ID
WSECTL_CLIENT_SECRET
WSECTL_SECRET_PASSPHRASE
```

Example:

```bash
WSECTL_ACCOUNT_URL=https://company.worksection.com WSECTL_ACCESS_TOKEN=... wsectl me --json
```
