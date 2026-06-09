# Output Contracts

JSON is the stable interface for scripts and agents. Table output is optimized for human readability and should not be parsed.

Command results are written to stdout or `--out`. Human diagnostics use stderr. Optional persistent history is written to its configured JSONL file,
not stdout, so enabling history does not contaminate JSON, NDJSON, raw, or file output.

## History Events

When enabled, local history is JSONL. Each line is one event with this shape:

```json
{
  "schema_version": "history.1",
  "timestamp": "2026-06-07T12:34:56.123456789Z",
  "command": "wsectl tasks list",
  "normalized_args": ["--project", "PROJECT_ID", "--json"],
  "action": "get_tasks",
  "profile": "default",
  "account_url": "https://company.worksection.com",
  "auth_type": "oauth2",
  "output": "json",
  "params": {"id_project": "PROJECT_ID"},
  "status": "ok",
  "exit_code": 0,
  "error_code": "",
  "duration_ms": 412,
  "count": 47,
  "truncated": false,
  "warnings": []
}
```

Timestamps are UTC RFC3339Nano strings. `history list --limit N` returns the latest N valid events.
`normalized_args` is the post-Cobra command view, not the literal shell command. It records flags in normalized form and keeps positional arguments
after flags. Secret values are redacted. `include_params` applies to both structured `params` and `--param` entries in `normalized_args`; ordinary
non-secret flags such as `--query` can still appear in `normalized_args`. History schema versions are independent from Worksection response
contract versions.

## Modes

```text
--output auto|json|yaml|table|ndjson|raw
--json
--yaml
--table
--ndjson
--raw
```

`auto` chooses table output for interactive terminals and JSON for non-terminal output.

## Success Envelope

```json
{
  "status": "ok",
  "data": [],
  "meta": {
    "action": "get_tasks",
    "profile": "default",
    "account_url": "https://company.worksection.com",
    "contract_version": "2026-06-07.1",
    "response_shape": "array",
    "count": 0,
    "truncated": false,
    "warnings": []
  }
}
```

Fields:

- `status`: `ok` on success.
- `data`: Worksection data returned by the action.
- `meta.action`: Worksection action or logical command action.
- `meta.profile`: selected profile name.
- `meta.account_url`: target Worksection account.
- `meta.contract_version`: version of the static response contract used by the binary.
- `meta.response_shape`: advisory shape for `data`: `array`, `object`, `composite`, `binary`, or `unknown`.
- `meta.count`: best-effort top-level item count.
- `meta.truncated`: true when the result may have hit a known API cap.
- `meta.warnings`: non-fatal completeness or behavior warnings.

Composite Worksection responses preserve their documented object fields instead of dropping everything except `data`. JSON and YAML keep aggregate
siblings such as `total`, `projects`, and `tasks`. Row-oriented modes (`--ndjson`, table output, `--limit`, and `--fields`) use the static contract's
`data_path` and `count_path` so list commands such as `costs list` still operate on primary rows.

## Static Response Contracts

Agents can inspect response contracts before making a Worksection request:

```bash
wsectl api schema get_tasks --json
wsectl tasks search --schema --json
```

The contract includes `response_shape`, `data_path`, `item_shape`, `conditional_fields`, `count_path`, and notes. It is advisory and versioned, not a
full JSON Schema.

## Error Envelope

```json
{
  "status": "error",
  "error": {
    "code": "rate_limited",
    "message": "Worksection API rate limit exceeded",
    "details": {}
  },
  "meta": {
    "action": "get_tasks",
    "profile": "default",
    "warnings": []
  }
}
```

Error codes are stable strings intended for automation.

When a machine-readable mode is selected, command setup failures such as invalid config, missing profile, missing credentials, and `doctor` failures
also use this envelope. They do not append an extra plaintext error line.

## NDJSON

`--ndjson` writes one top-level data item per line for array responses:

```bash
wsectl tasks all --ndjson --out /tmp/tasks.ndjson
```

Use this when downstream tools process one record at a time.

## Raw

`--raw` prints the exact Worksection response body for API calls. It skips the `wsectl` envelope and is useful for debugging schema changes. Use
`--raw --out FILE` for large raw responses. If Worksection returns an API error inside an HTTP 200 JSON body, raw mode still prints or writes the
exact body and exits with the Worksection API error code.

Because raw mode emits the upstream bytes verbatim, the transform flags `--fields`, `--limit`, and `--jq` cannot be applied. If any of them are
present alongside `--raw`, `wsectl` emits a one-line warning to stderr per flag and proceeds. Use `--quiet` to suppress the warnings, or pipe the
raw body into `jq` for downstream processing.

## Field Projection

```bash
wsectl projects list --json --fields id,name,status
wsectl tasks search --json --fields id,name,project.name
wsectl projects events --project PROJECT_ID --period 1d --table --fields action,date_added
```

`--fields` supports dotted paths and keeps warnings in `meta.warnings` when requested fields are missing or unknown to the static action contract. It
applies to JSON, YAML, NDJSON, and table output. With `--table` the field list also fixes the column set and order. `--raw` ignores `--fields`
(see [Raw](#raw)).

Without `--fields`, table output uses the action's curated columns (`table_columns` in the action schema). Inspect them with
`wsectl api schema ACTION --json` or `wsectl COMMAND --schema --yaml`.

Use `--jq` for richer transforms:

```bash
wsectl projects list --json --jq '.data[] | {id, name}'
```

## Client-Side Limits

```bash
wsectl tasks all --json --limit 100
```

`--limit` slices array output client-side after the API response is received. It does not reduce Worksection server work and adds a warning when
applied.

## Exit Codes

```text
0 success
1 general error
2 usage or validation error
3 authentication error
4 authorization or permission error
5 network error
6 Worksection API error
7 rate limit error
8 partial or truncated result when --fail-on-truncated is set
```
