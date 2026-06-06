# Recipes

## Set Up Persistent Desktop OAuth

```bash
wsectl profiles add default \
  --account-url https://company.worksection.com \
  --auth-type oauth2

wsectl profiles use default

# Set WSECTL_CLIENT_ID and WSECTL_CLIENT_SECRET in your shell first.
wsectl auth login --client-id "$WSECTL_CLIENT_ID"

wsectl doctor --api
```

After this, read commands can run without credential environment variables:

```bash
wsectl me --json
wsectl projects list --json
```

## Manage Multiple Profiles

```bash
wsectl profiles add agency --account-url https://agency.worksection.com --auth-type oauth2
wsectl profiles add client --account-url https://client.worksection.com --auth-type oauth2
wsectl --profile agency projects list --json
wsectl profiles use client
wsectl profiles list --json
```

## Store An Admin API Token

```bash
wsectl profiles add admin \
  --account-url https://company.worksection.com \
  --auth-type admin_token \
  --secret-ref keyring:wsectl/admin

# Set WSECTL_ADMIN_TOKEN in your shell first.
wsectl --profile admin auth login
wsectl --profile admin doctor --api
```

## Use A Portable Encrypted Secret File

```bash
export WSECTL_SECRET_PASSPHRASE="use-a-password-manager-value"

wsectl profiles add portable \
  --account-url https://company.worksection.com \
  --auth-type oauth2 \
  --secret-ref encrypted-file:$HOME/.config/wsectl/secrets/portable.json

# Set WSECTL_CLIENT_ID and WSECTL_CLIENT_SECRET in your shell first.
wsectl --profile portable auth login --client-id "$WSECTL_CLIENT_ID"
```

## Confirm Authentication

```bash
wsectl auth status --json
wsectl me --json
```

## Export Active Projects

```bash
wsectl projects list --status active --extra text,options,users --json --out /tmp/projects.json
jq '.data[] | {id, name, status}' /tmp/projects.json
```

## Export All Active Tasks

```bash
wsectl tasks all --status active --extra text,files,comments --json --out /tmp/tasks.json
jq '.meta' /tmp/tasks.json
```

Use `--fail-on-truncated` when completeness matters:

```bash
wsectl tasks all --status active --json --fail-on-truncated --out /tmp/tasks.json
```

## Search Tasks

Simple text search:

```bash
wsectl tasks search --query "invoice" --json
```

Project-scoped search:

```bash
wsectl tasks search --project 123 --query "invoice" --json
```

Advanced filter:

```bash
wsectl tasks search --filter "name has 'Report'" --json
```

## Inspect A Project Team

```bash
wsectl projects team 123 --json
```

## Pull Task Discussion

```bash
wsectl tasks discussion 456 --json
wsectl comments list 456 --extra files --json
```

## Download Files

```bash
wsectl files list --project 123 --json --out /tmp/files.json
wsectl files download 789 --out ./attachment.bin
```

## Costs For A Date Range

```bash
wsectl costs list --project 123 --start 01.05.2026 --end 31.05.2026 --json
wsectl costs total --project 123 --start 01.05.2026 --end 31.05.2026 --json
```

## Use Environment Credentials In CI

```bash
export WSECTL_ACCOUNT_URL=https://company.worksection.com
export WSECTL_ACCESS_TOKEN=...
wsectl projects list --json --out projects.json
```

## Discover Lower-Level API Support

```bash
wsectl api actions --json
wsectl api schema get_files --json
wsectl api call get_files --param id_project=123 --json
```
