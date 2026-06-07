# Agent Usage

This document is for coding agents and automation that call `wsectl` from scripts.

## Start Here

Use the guide compiled into the current binary:

```bash
wsectl help agent --full
wsectl help agent --full --json
```

The JSON form retains `topic` and `content` and adds `guide_format_version`, structured sections, and the command catalog.

## Rules

- Prefer `--json` for normal responses.
- Prefer `--ndjson` for large arrays that will be streamed into another tool.
- Do not parse table output.
- Use `--out FILE` for large responses.
- Check `meta.truncated` and `meta.warnings` before trusting completeness.
- Use `--profile NAME` when account context matters.
- Use `wsectl api call ACTION` for lower-level read-only API access.
- Avoid request loops. Worksection documents a 1 request/second limit.
- Never print tokens unless the user explicitly asks for token material.

## Good Defaults

```bash
wsectl projects list --json
wsectl tasks search --query "invoice" --json
wsectl tasks all --extra text,files --json --out /tmp/tasks.json
wsectl files list --project 123 --json --out /tmp/files.json
```

For endpoint coverage discovery:

```bash
wsectl commands --json
wsectl api actions --json
wsectl api schema get_tasks --json
wsectl tasks search --schema --json
wsectl doctor --json
```

`commands --json` is the authoritative discovery contract. It includes explicit `category`, `actions`, `output_modes`, `agent_notes`, `auth_required`,
`read_only`, usage, flags, and examples. `api schema ACTION --json` and command `--schema --json` return static, advisory response contracts without
calling Worksection. Use them before selecting `--fields` or writing a `--jq` expression for an unfamiliar response.

## Compatibility Notes

First-class commands normalize known Worksection quirks. Raw `api call` uses raw API parameter names and values.

- `projects list --status archived` sends `filter=archive`.
- `tasks search --query TEXT` sends a Worksection `filter`, not a `search` parameter.
- `costs --timer true` sends `is_timer=true`.
- `files images` filters image files client-side.
- API calls use Worksection's documented query-parameter POST mode. Live probes showed form-encoded API bodies return `invalid JSON`; do not assume
  form bodies are supported.

For lower-level calls:

```bash
wsectl api call get_users_schedule \
  --param datestart=01.05.2026 \
  --param dateend=31.05.2026 \
  --json
```

## Output Contract

Success responses use this shape:

```json
{
  "status": "ok",
  "data": [],
  "meta": {
    "action": "get_tasks",
    "profile": "default",
    "account_url": "https://company.worksection.com",
    "count": 0,
    "truncated": false,
    "warnings": []
  }
}
```

Errors use this shape:

```json
{
  "status": "error",
  "error": {
    "code": "authentication",
    "message": "profile \"default\" not found",
    "details": {}
  },
  "meta": {
    "action": "get_tasks",
    "profile": "default",
    "warnings": []
  }
}
```

## Large Response Pattern

```bash
wsectl tasks all --extra text,files --json --out /tmp/tasks.json
jq '.meta.truncated, .meta.warnings' /tmp/tasks.json
jq '.data[] | {id, name, status}' /tmp/tasks.json
```

Use `--fail-on-truncated` when the task requires complete data:

```bash
wsectl tasks all --json --fail-on-truncated --out /tmp/tasks.json
```

## Mutation Safety

The MVP is read-only. If an agent tries a known mutation through `api call`, `wsectl` exits with a usage error and prints:

```text
This action changes Worksection data and is blocked in the read-only build.
```
