# API Coverage

The MVP targets full read-only coverage for the documented Worksection API areas listed below. Most actions have first-class commands; all registered
read-only actions are available through `wsectl api call`.

Source precedence for wire behavior:

1. Official Postman collections and live read-only probes.
2. Official web docs.
3. Local compatibility notes compiled into `wsectl`.

This precedence exists because Worksection web docs and Postman/live API behavior can disagree.

## Wire Mode

`wsectl` currently sends Worksection API calls as `POST` requests with API parameters in the query string, matching the documented examples and
verified live behavior. Do not change this to `application/x-www-form-urlencoded`: live probes against `/api/oauth2` returned Worksection's
`invalid JSON` error for form bodies.

Live probes also showed JSON request bodies can work for selected OAuth API reads, but JSON-body mode is not implemented in this build. Very long
filters can still hit Worksection/request URL limits because the default wire mode keeps parameters in the query string.

## Read-Only Actions

| Action                   | First-class command                                                      |
| ------------------------ | ------------------------------------------------------------------------ |
| `me`                     | `wsectl me`                                                              |
| `get_users`              | `wsectl users list`                                                      |
| `get_user_groups`        | `wsectl users groups`                                                    |
| `get_contacts`           | `wsectl users contacts`                                                  |
| `get_contact_groups`     | `wsectl users contact-groups`                                            |
| `get_users_schedule`     | `wsectl users schedule`                                                  |
| `get_projects`           | `wsectl projects list`                                                   |
| `get_project`            | `wsectl projects get`, `wsectl projects team`                            |
| `get_project_groups`     | `wsectl projects groups`                                                 |
| `get_events`             | `wsectl projects events`                                                 |
| `get_all_tasks`          | `wsectl tasks all`                                                       |
| `get_tasks`              | `wsectl tasks list`                                                      |
| `get_task`               | `wsectl tasks get`, `subtasks`, `relations`, `subscribers`, `discussion` |
| `search_tasks`           | `wsectl tasks search`                                                    |
| `get_comments`           | `wsectl comments list`                                                   |
| `get_task_tags`          | `wsectl tags task list`                                                  |
| `get_task_tag_groups`    | `wsectl tags task groups`                                                |
| `get_project_tags`       | `wsectl tags project list`                                               |
| `get_project_tag_groups` | `wsectl tags project groups`                                             |
| `get_costs`              | `wsectl costs list`                                                      |
| `get_costs_total`        | `wsectl costs total`                                                     |
| `get_timers`             | `wsectl timers list`                                                     |
| `get_my_timer`           | `wsectl timers mine`                                                     |
| `get_files`              | `wsectl files list`, `wsectl files images`                               |
| `download`               | `wsectl files download`                                                  |
| `get_webhooks`           | `wsectl webhooks list`                                                   |

## Escape Hatch

```bash
wsectl api call ACTION --param key=value --json
```

Unknown actions require `--allow-unknown`. Known mutations remain blocked.

`api call` uses raw API parameter names and values. First-class commands may normalize known quirks for humans and agents.

## Static Schemas

```bash
wsectl api schema get_projects --json
wsectl projects list --schema --json
```

Schemas are static, advisory contracts compiled into the binary. They include documented parameters, enum values, auth modes, OAuth scopes, response
shape, known fields, conditional `extra` fields, count path, and compatibility notes. They do not infer fields from live account data and are not full
JSON Schema.

## Compatibility Notes

- `get_projects`: first-class `--status archived` maps to API `filter=archive`. Raw `api call` users should pass `--param filter=archive`.
- `get_events`: `period` is required; `id_project` is optional.
- `get_tasks` and `get_all_tasks`: completed tasks are not exposed through list filters. Use `wsectl tasks search --status done`.
- `search_tasks`: CLI `--query TEXT` is translated to `filter=name has 'TEXT'`; the CLI does not send an undocumented `search` parameter. The
  documented `id_task` search selector is exposed as `wsectl tasks search --task TASK_ID`.
- `get_costs` and `get_costs_total`: public `--timer` maps to API `is_timer`. `costs list` returns the cost entries as an array at `data` with the
  server-side `total` summary in `meta.aggregate`; `costs total` returns the aggregate bundle (`total`, optional `projects`/`tasks`) as its `data` object.
- `get_files`: callers must specify exactly one selector, `id_project` or `id_task`.
- `files images`: image filtering is client-side because the Postman/live contract does not document a server-side image filter.
- `download`: responses may be JSON URL objects, redirects, or direct binary content. Bearer credentials are forwarded only to HTTPS URLs on the
  configured Worksection account host; cross-host or insecure URLs return structured blocked-download errors.
- `get_webhooks`: access may require an administrative token or explicit administrative OAuth scope depending on the Worksection app settings. Current
  docs show `events`, `status`, and optional `projects` fields.
- `get_users_schedule`: docs show `data` as an object keyed by user with nested `schedule`, not a flat list of dates.
- `get_timers` and `get_my_timer`: docs use `date_started`, `user_from`, and nested `task` fields. `get_my_timer` can vary when no timer is active.
- Tag and project-group contracts use documented `title` fields; `name` is retained as a known compatibility field for field-selection warnings.

## Blocked Mutation Actions

The read-only build recognizes documented mutation names and blocks them before sending a request. The user-facing error is:

```text
This action changes Worksection data and is blocked in the read-only build.
```

Examples of blocked actions include `post_task`, `update_task`, `post_project`, `post_comment`, `add_costs`, `start_my_timer`, and `add_webhook`.

The web docs and Postman collection disagree on the project-folder mutation name: the web docs show `add_project_group`, while the current Postman
collection sends `action=add_project_groups`. Both names are treated as mutations and blocked locally.
