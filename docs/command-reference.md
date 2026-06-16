# Command Reference

Generated from the command metadata compiled into `wsectl`. Do not edit by hand.

## `wsectl`

Unofficial command-line client for Worksection

Unofficial command-line client for Worksection.

This is an unofficial tool and is not affiliated with Worksection.

**Usage:** `wsectl [flags]`

**Command metadata:**

- Category: `discovery`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `text`

**Command flags:**

- `--account-url`
- `--config`
- `--debug`
- `--fail-on-truncated`
- `--fields`
- `--jq`
- `--json`
- `--limit`
- `--ndjson`
- `--out`
- `--output`
- `--profile`
- `--quiet`
- `--rate-limit`
- `--raw`
- `--schema`
- `--table`
- `--timeout`
- `--verbose`
- `--yaml`

**Examples:**

```bash
wsectl
wsectl help agent --full
```

## `wsectl api`

Low-level Worksection API access

**Usage:** `wsectl api [flags]`

**Command metadata:**

- Category: `api`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl api --help
```

## `wsectl api actions`

List known Worksection API actions

**Usage:** `wsectl api actions [flags]`

**Command metadata:**

- Category: `api`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `json`

**Examples:**

```bash
wsectl api actions --json
```

## `wsectl api call`

Call a read-only Worksection API action

**Usage:** `wsectl api call ACTION [flags]`

**Command metadata:**

- Category: `api`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `dynamic`

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--allow-unknown`
- `--param`
- `--params-json`

**Examples:**

```bash
wsectl api call get_users_schedule --param datestart=01.05.2026 --param dateend=31.05.2026 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl api schema`

Show known action parameters

**Usage:** `wsectl api schema ACTION [flags]`

**Command metadata:**

- Category: `api`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `json`

**Examples:**

```bash
wsectl api schema get_tasks --json
```

## `wsectl auth`

Authenticate with Worksection

**Usage:** `wsectl auth [flags]`

**Command metadata:**

- Category: `auth`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl auth --help
```

## `wsectl auth login`

Start OAuth login or store manually supplied credentials

**Usage:** `wsectl auth login [flags]`

**Command metadata:**

- Category: `auth`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `auto`, `json`, `yaml`, `table`

**Command flags:**

- `--access-token`
- `--admin-token`
- `--admin-token-stdin`
- `--callback-cert`
- `--callback-host`
- `--callback-key`
- `--callback-port`
- `--client-id`
- `--client-secret`
- `--client-secret-stdin`
- `--code`
- `--login-timeout`
- `--manual-code`
- `--no-browser`
- `--refresh-token`
- `--scope`

**Examples:**

```bash
wsectl auth login --client-id "$WSECTL_CLIENT_ID"
```

## `wsectl auth logout`

Delete stored credentials for the active profile

**Usage:** `wsectl auth logout [flags]`

**Command metadata:**

- Category: `auth`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Output modes:** `auto`, `json`, `yaml`, `table`

**Examples:**

```bash
wsectl auth logout
```

## `wsectl auth refresh`

Refresh OAuth token when stored refresh token is available

**Usage:** `wsectl auth refresh [flags]`

**Command metadata:**

- Category: `auth`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Output modes:** `auto`, `json`, `yaml`, `table`

**Examples:**

```bash
wsectl auth refresh
```

## `wsectl auth status`

Show auth status

**Usage:** `wsectl auth status [flags]`

**Command metadata:**

- Category: `auth`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `auto`, `json`, `yaml`, `table`

**Examples:**

```bash
wsectl auth status --json
```

## `wsectl commands`

List public commands

**Usage:** `wsectl commands [flags]`

**Command metadata:**

- Category: `discovery`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `json`

**Examples:**

```bash
wsectl commands --json
```

## `wsectl comments`

Read comments

**Usage:** `wsectl comments [flags]`

**Command metadata:**

- Category: `comments`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl comments --help
```

## `wsectl comments list`

List task comments

**Usage:** `wsectl comments list TASK_ID [flags]`

**Command metadata:**

- Category: `comments`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_comments`

- `get_comments` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--extra`

**Examples:**

```bash
wsectl comments list 456 --extra files --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl completion`

Generate shell completion scripts

Generate a completion script from the current wsectl command tree.

**Usage:** `wsectl completion [flags]`

**Command metadata:**

- Category: `completion`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `text`

**Examples:**

```bash
wsectl completion --help
```

## `wsectl completion bash`

Generate the completion script for bash

**Usage:** `wsectl completion bash`

**Command metadata:**

- Category: `completion`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `text`

**Examples:**

```bash
wsectl completion bash > /etc/bash_completion.d/wsectl
```

## `wsectl completion fish`

Generate the completion script for fish

**Usage:** `wsectl completion fish`

**Command metadata:**

- Category: `completion`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `text`

**Examples:**

```bash
wsectl completion fish > ~/.config/fish/completions/wsectl.fish
```

## `wsectl completion powershell`

Generate the completion script for powershell

**Usage:** `wsectl completion powershell`

**Command metadata:**

- Category: `completion`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `text`

**Examples:**

```bash
wsectl completion powershell > wsectl.ps1
```

## `wsectl completion zsh`

Generate the completion script for zsh

**Usage:** `wsectl completion zsh`

**Command metadata:**

- Category: `completion`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `text`

**Examples:**

```bash
wsectl completion zsh > "${fpath[1]}/_wsectl"
```

## `wsectl costs`

Read costs

**Usage:** `wsectl costs [flags]`

**Command metadata:**

- Category: `costs`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl costs --help
```

## `wsectl costs list`

Read costs

**Usage:** `wsectl costs list [flags]`

**Command metadata:**

- Category: `costs`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_costs`

- `get_costs` response shape: `array`; count path: `data`.
- `get_costs` compatibility: The public CLI flag is --timer, but the Worksection API parameter is is_timer.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--end`
- `--extra`
- `--filter`
- `--project`
- `--start`
- `--task`
- `--timer`

**Examples:**

```bash
wsectl costs list --project 123 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl costs total`

Read costs

**Usage:** `wsectl costs total [flags]`

**Command metadata:**

- Category: `costs`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_costs_total`

- `get_costs_total` response shape: `object`; count path: `data`.
- `get_costs_total` compatibility: The public CLI flag is --timer, but the Worksection API parameter is is_timer.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--end`
- `--extra`
- `--filter`
- `--project`
- `--start`
- `--task`
- `--timer`

**Examples:**

```bash
wsectl costs total --project 123 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl docs`

Documentation helpers

**Usage:** `wsectl docs [flags]`

**Command metadata:**

- Category: `docs`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl docs --help
```

## `wsectl docs generate`

Generate command reference

**Usage:** `wsectl docs generate [flags]`

**Command metadata:**

- Category: `docs`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `markdown`

**Command flags:**

- `--out`

**Examples:**

```bash
wsectl docs generate --out docs/command-reference.md
```

## `wsectl doctor`

Diagnose configuration, credentials, and optional API connectivity

Run local configuration and credential checks. Pass --api to also perform one authenticated read request.

**Usage:** `wsectl doctor [flags]`

**Command metadata:**

- Category: `diagnostics`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `me`

- `me` response shape: `object`; count path: `data`.

**Output modes:** `text`, `json`

**Command flags:**

- `--api`

**Examples:**

```bash
wsectl doctor
wsectl doctor --json
wsectl doctor --api --json
```

**Agent notes:**

- The me action is called only when --api is set.

## `wsectl files`

Read and download files

**Usage:** `wsectl files [flags]`

**Command metadata:**

- Category: `files`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl files --help
```

## `wsectl files download`

Download a file

**Usage:** `wsectl files download FILE_ID [flags]`

**Command metadata:**

- Category: `files`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `download`

- `download` response shape: `binary`; count path: `body`.

**Output modes:** `file`, `stdout`

**Command flags:**

- `--out`

**Examples:**

```bash
wsectl files download 789 --out ./attachment.bin
```

**Agent notes:**

- Use --out FILE for binary content.

## `wsectl files images`

List image files

**Usage:** `wsectl files images [flags]`

**Command metadata:**

- Category: `files`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_files`

- `get_files` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--project`
- `--task`

**Examples:**

```bash
wsectl files images --task 456 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl files list`

List files

**Usage:** `wsectl files list [flags]`

**Command metadata:**

- Category: `files`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_files`

- `get_files` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--project`
- `--task`

**Examples:**

```bash
wsectl files list --project 123 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl files task-attachments`

List task attachments

**Usage:** `wsectl files task-attachments TASK_ID [flags]`

**Command metadata:**

- Category: `files`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_task`

- `get_task` response shape: `object`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl files task-attachments 456 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl help`

Show detailed help

**Usage:** `wsectl help [topic|command] [flags]`

**Command metadata:**

- Category: `discovery`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `text`, `json`

**Command flags:**

- `--full`
- `--json`

**Examples:**

```bash
wsectl help agent --full
wsectl help agent --json
```

## `wsectl history`

Inspect local command history

Inspect optional local JSONL command history. History is disabled by default and never stores tokens or full Worksection responses.

**Usage:** `wsectl history [flags]`

**Command metadata:**

- Category: `history`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl history --help
```

## `wsectl history clear`

Clear local command history

Delete the configured local command history JSONL file, or keep only the latest N entries with --keep.
This command uses a local lock to avoid racing concurrent history writes and does not write a replacement history event.

**Usage:** `wsectl history clear [flags]`

**Command metadata:**

- Category: `history`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `auto`, `json`, `yaml`, `table`

**Command flags:**

- `--keep`

**Examples:**

```bash
wsectl history clear --keep 1000
wsectl history clear
```

## `wsectl history list`

List local command history

List optional local command history entries from the configured JSONL history file.

**Usage:** `wsectl history list [flags]`

**Command metadata:**

- Category: `history`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`

**Examples:**

```bash
wsectl history list --json --limit 20
```

## `wsectl history path`

Show local command history path

Show whether optional local command history is enabled and which JSONL file path is used.

**Usage:** `wsectl history path [flags]`

**Command metadata:**

- Category: `history`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `auto`, `json`, `yaml`, `table`

**Examples:**

```bash
wsectl history path --json
```

## `wsectl me`

Get authorized user info

**Usage:** `wsectl me [flags]`

**Command metadata:**

- Category: `identity`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `me`

- `me` response shape: `object`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl me --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl profiles`

Manage Worksection profiles

**Usage:** `wsectl profiles [flags]`

**Command metadata:**

- Category: `profiles`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl profiles --help
```

## `wsectl profiles add`

Add or update a profile

**Usage:** `wsectl profiles add NAME [flags]`

**Command metadata:**

- Category: `profiles`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Command flags:**

- `--account-url`
- `--auth-type`
- `--secret-ref`

**Examples:**

```bash
wsectl profiles add default --account-url https://company.worksection.com --auth-type oauth2
```

## `wsectl profiles list`

List profiles

**Usage:** `wsectl profiles list [flags]`

**Command metadata:**

- Category: `profiles`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`

**Examples:**

```bash
wsectl profiles list --json
```

## `wsectl profiles remove`

Remove a profile

**Usage:** `wsectl profiles remove NAME [flags]`

**Command metadata:**

- Category: `profiles`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl profiles remove old-account
```

## `wsectl profiles show`

Show a profile

**Usage:** `wsectl profiles show [NAME] [flags]`

**Command metadata:**

- Category: `profiles`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `auto`, `json`, `yaml`, `table`

**Examples:**

```bash
wsectl profiles show default --json
```

## `wsectl profiles use`

Set current profile

**Usage:** `wsectl profiles use NAME [flags]`

**Command metadata:**

- Category: `profiles`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl profiles use default
```

## `wsectl projects`

Read projects

**Usage:** `wsectl projects [flags]`

**Command metadata:**

- Category: `projects`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl projects --help
```

## `wsectl projects events`

Get project events

**Usage:** `wsectl projects events [flags]`

**Command metadata:**

- Category: `projects`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_events`

- `get_events` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--period`
- `--project`

**Examples:**

```bash
wsectl projects events --project 123 --period 7d --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl projects get`

Get a project

**Usage:** `wsectl projects get PROJECT_ID [flags]`

**Command metadata:**

- Category: `projects`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_project`

- `get_project` response shape: `object`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--extra`

**Examples:**

```bash
wsectl projects get 123 --extra text,users --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl projects groups`

List project groups

**Usage:** `wsectl projects groups [flags]`

**Command metadata:**

- Category: `projects`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_project_groups`

- `get_project_groups` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl projects groups --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl projects list`

List projects

**Usage:** `wsectl projects list [flags]`

**Command metadata:**

- Category: `projects`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_projects`

- `get_projects` response shape: `array`; count path: `data`.
- `get_projects` compatibility: Official docs use archived in some places, but Postman/live API expects filter=archive.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--extra`
- `--status`

**Examples:**

```bash
wsectl projects list --status active --extra text,options,users --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl projects team`

Get project team

**Usage:** `wsectl projects team PROJECT_ID [flags]`

**Command metadata:**

- Category: `projects`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_project`

- `get_project` response shape: `object`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl projects team 123 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl tags`

Read task and project tags

**Usage:** `wsectl tags [flags]`

**Command metadata:**

- Category: `tags`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl tags --help
```

## `wsectl tags project`

Read project tags

**Usage:** `wsectl tags project [flags]`

**Command metadata:**

- Category: `tags`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl tags project --help
```

## `wsectl tags project groups`

List tag groups

**Usage:** `wsectl tags project groups [flags]`

**Command metadata:**

- Category: `tags`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_project_tag_groups`

- `get_project_tag_groups` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--access`
- `--type`

**Examples:**

```bash
wsectl tags project groups --type status --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl tags project list`

List tags

**Usage:** `wsectl tags project list [flags]`

**Command metadata:**

- Category: `tags`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_project_tags`

- `get_project_tags` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--access`
- `--group`
- `--type`

**Examples:**

```bash
wsectl tags project list --type label --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl tags task`

Read task tags

**Usage:** `wsectl tags task [flags]`

**Command metadata:**

- Category: `tags`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl tags task --help
```

## `wsectl tags task groups`

List tag groups

**Usage:** `wsectl tags task groups [flags]`

**Command metadata:**

- Category: `tags`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_task_tag_groups`

- `get_task_tag_groups` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--access`
- `--type`

**Examples:**

```bash
wsectl tags task groups --type status --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl tags task list`

List tags

**Usage:** `wsectl tags task list [flags]`

**Command metadata:**

- Category: `tags`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_task_tags`

- `get_task_tags` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--access`
- `--group`
- `--type`

**Examples:**

```bash
wsectl tags task list --type label --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl tasks`

Read and search tasks

**Usage:** `wsectl tasks [flags]`

**Command metadata:**

- Category: `tasks`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl tasks --help
```

## `wsectl tasks all`

List all account tasks

List all account tasks. Worksection can cap large responses at 10000 records; check meta.truncated and meta.warnings.

**Usage:** `wsectl tasks all [flags]`

**Command metadata:**

- Category: `tasks`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_all_tasks`

- `get_all_tasks` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--extra`
- `--status`

**Examples:**

```bash
wsectl tasks all --extra text,files --json --out /tmp/tasks.json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl tasks discussion`

Get task comments

**Usage:** `wsectl tasks discussion TASK_ID [flags]`

**Command metadata:**

- Category: `tasks`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_task`

- `get_task` response shape: `object`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl tasks discussion 456 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl tasks get`

Get a task

**Usage:** `wsectl tasks get TASK_ID [flags]`

**Command metadata:**

- Category: `tasks`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_task`

- `get_task` response shape: `object`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--extra`

**Examples:**

```bash
wsectl tasks get 456 --extra text,files --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl tasks list`

List project tasks

**Usage:** `wsectl tasks list [flags]`

**Command metadata:**

- Category: `tasks`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_tasks`

- `get_tasks` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--extra`
- `--project`
- `--status`

**Examples:**

```bash
wsectl tasks list --project 123 --status active --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl tasks relations`

Get task relations

**Usage:** `wsectl tasks relations TASK_ID [flags]`

**Command metadata:**

- Category: `tasks`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_task`

- `get_task` response shape: `object`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl tasks relations 456 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl tasks search`

Search tasks

Search tasks by simple query or advanced Worksection filter. Prefer --json for scripts and use --out for large results.

**Usage:** `wsectl tasks search [flags]`

**Command metadata:**

- Category: `tasks`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `search_tasks`

- `search_tasks` response shape: `array`; count path: `data`.
- `search_tasks` compatibility: The CLI translates --query TEXT to filter=name has 'TEXT'. The raw API parameter is filter, not search.
- `search_tasks` compatibility: search_tasks needs at least one of filter, id_project, id_task, email_user_to, email_user_from; status and extra are
  modifiers, not search criteria.
- `search_tasks` compatibility: The advanced filter grammar supports name and the date fields dateadd, datestart, dateend, dateclose (the underscore
  response-field forms date_added/date_closed are also accepted) with operators has, <, >, and (e.g. "dateadd > '01.06.2026' and dateadd <
  '12.06.2026'"), enabling server-side date-range filtering. Unsupported fields such as status, tag, or priority make Worksection reject the filter
  with a misleading "Field is required: filter".

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--assignee`
- `--author`
- `--extra`
- `--filter`
- `--project`
- `--query`
- `--status`
- `--task`

**Examples:**

```bash
wsectl tasks search --query invoice --json
wsectl tasks search --filter "name has 'Report'" --json
wsectl tasks search --project 123 --assignee user@example.com --status active --json
wsectl tasks search --task 456 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl tasks subscribers`

Get task subscribers

**Usage:** `wsectl tasks subscribers TASK_ID [flags]`

**Command metadata:**

- Category: `tasks`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_task`

- `get_task` response shape: `object`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl tasks subscribers 456 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl tasks subtasks`

Get task subtasks

**Usage:** `wsectl tasks subtasks TASK_ID [flags]`

**Command metadata:**

- Category: `tasks`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_task`

- `get_task` response shape: `object`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl tasks subtasks 456 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl timers`

Read timers

**Usage:** `wsectl timers [flags]`

**Command metadata:**

- Category: `timers`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl timers --help
```

## `wsectl timers list`

List enabled member timers

**Usage:** `wsectl timers list [flags]`

**Command metadata:**

- Category: `timers`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_timers`

- `get_timers` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl timers list --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl timers mine`

Get current user timer

**Usage:** `wsectl timers mine [flags]`

**Command metadata:**

- Category: `timers`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_my_timer`

- `get_my_timer` response shape: `object`; count path: `data`.
- `get_my_timer` compatibility: Official docs show get_my_timer as an OAuth-only current-user timer response; the no-active-timer shape can vary by
  account state.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl timers mine --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl users`

Read users, groups, contacts, and schedules

**Usage:** `wsectl users [flags]`

**Command metadata:**

- Category: `users`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl users --help
```

## `wsectl users contact-groups`

List contact groups

**Usage:** `wsectl users contact-groups [flags]`

**Command metadata:**

- Category: `users`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_contact_groups`

- `get_contact_groups` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl users contact-groups --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl users contacts`

List contacts

**Usage:** `wsectl users contacts [flags]`

**Command metadata:**

- Category: `users`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_contacts`

- `get_contacts` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl users contacts --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl users groups`

List user groups

**Usage:** `wsectl users groups [flags]`

**Command metadata:**

- Category: `users`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_user_groups`

- `get_user_groups` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl users groups --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl users list`

List users

**Usage:** `wsectl users list [flags]`

**Command metadata:**

- Category: `users`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_users`

- `get_users` response shape: `array`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl users list --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl users schedule`

List users' non-working days

**Usage:** `wsectl users schedule [flags]`

**Command metadata:**

- Category: `users`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_users_schedule`

- `get_users_schedule` response shape: `object`; count path: `data`.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Command flags:**

- `--end`
- `--start`
- `--users`

**Examples:**

```bash
wsectl users schedule --start 01.05.2026 --end 31.05.2026 --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.

## `wsectl version`

Print version information

**Usage:** `wsectl version [flags]`

**Command metadata:**

- Category: `discovery`
- Authentication required: `false`
- Read-only: `true`
- Output support: `true`

**Output modes:** `text`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl version
wsectl version --json
```

## `wsectl webhooks`

Read webhooks

**Usage:** `wsectl webhooks [flags]`

**Command metadata:**

- Category: `webhooks`
- Authentication required: `false`
- Read-only: `true`
- Output support: `false`

**Examples:**

```bash
wsectl webhooks --help
```

## `wsectl webhooks list`

List webhooks

**Usage:** `wsectl webhooks list [flags]`

**Command metadata:**

- Category: `webhooks`
- Authentication required: `true`
- Read-only: `true`
- Output support: `true`

**Worksection actions:** `get_webhooks`

- `get_webhooks` response shape: `array`; count path: `data`.
- `get_webhooks` compatibility: Webhook access may require an administrative token or an explicit administrative OAuth scope depending on the
  Worksection app settings.

**Output modes:** `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`

**Examples:**

```bash
wsectl webhooks list --json
```

**Agent notes:**

- Prefer --json or --ndjson for automation.
