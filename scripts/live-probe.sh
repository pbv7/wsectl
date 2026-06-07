#!/usr/bin/env bash
#
# Live release probe. Exercises the wsectl binary end-to-end against a real
# Worksection account before tagging a release. Read-only: no write actions
# are issued and the build itself blocks them.
#
# Credentials:
#   Uses whatever auth the wsectl binary already resolves. If you have a
#   configured profile with stored OAuth or admin-token credentials
#   (e.g. set up via `wsectl auth login`), nothing else is required.
#   For one-shot runs, you can override with:
#     WSECTL_ACCOUNT_URL    Worksection account base URL
#     WSECTL_ACCESS_TOKEN   OAuth access token (env auth)
#     WSECTL_ADMIN_TOKEN    Admin API key (env auth)
#
# Optional environment:
#   WSECTL                Command used to invoke the CLI. Defaults to the
#                         built binary at dist/wsectl (auto-built if missing).
#                         Use a release binary to verify packaged artifacts.
#                         Note: `go run` cannot be used here because it does
#                         not propagate child exit codes faithfully (always
#                         exits 1 for any non-zero subprocess exit).
#   WSECTL_PROBE_PROJECT  Project ID to use instead of the first discovered.
#   WSECTL_PROBE_TASK     Task ID to use instead of the first discovered.
#   WSECTL_PROBE_FILE     File ID to use instead of the first discovered.
#   WSECTL_HISTORY        Defaults to 0 so release probes do not write to a
#                         user's persistent local history unless explicitly
#                         overridden.
#
# Exit codes:
#   0  all probes passed
#   1  at least one probe failed

set -u

WSECTL="${WSECTL:-dist/wsectl}"
WSECTL_HISTORY="${WSECTL_HISTORY:-0}"
export WSECTL_HISTORY
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Auto-build the default binary if it's missing so `make live-probe` works
# from a clean checkout.
if [[ "$WSECTL" == "dist/wsectl" && ! -x "$WSECTL" ]]; then
  echo "building $WSECTL ..." >&2
  go build -o "$WSECTL" ./cmd/wsectl
fi

PASS=0
FAIL=0
SKIP=0

pluralize() {
  local n="$1" singular="$2" plural="$3"
  if [[ "$n" == "1" ]]; then printf '%s %s' "$n" "$singular"
  else                       printf '%s %s' "$n" "$plural"
  fi
}

# probe_info FILE  -> safe per-probe summary extracted from the response body.
# Surfaces only metadata (count, truncated, warnings, healthy, bytes, lines),
# never data values. Falls back to line/byte counts for non-JSON outputs
# (table, ndjson, raw). Empty for outputs written to --out (no stdout).
probe_info() {
  local file="$1"
  local bytes lines
  bytes="$(wc -c < "$file" | tr -d ' ')"
  lines="$(wc -l < "$file" | tr -d ' ')"

  # No stdout — likely a --out FILE write. Leave the info empty so the
  # caller can print its own metric (e.g. download size on the next line).
  if [[ "$bytes" -eq 0 ]]; then
    return 0
  fi

  local count truncated warnings healthy version
  count="$(sed -n 's/.*"count": *\([0-9][0-9]*\).*/\1/p'           "$file" | head -1)"
  truncated="$(sed -n 's/.*"truncated": *\(true\|false\).*/\1/p'   "$file" | head -1)"
  warnings="$(sed -n 's/.*"warnings": *\[\([^]]*\)\].*/\1/p'       "$file" | head -1)"
  healthy="$(sed -n 's/.*"healthy": *\(true\|false\).*/\1/p'       "$file" | head -1)"
  version="$(sed -n 's/.*"version": *"\([0-9][^"]*\)".*/\1/p'      "$file" | head -1)"

  local parts=()
  if [[ -n "$count" ]]; then
    parts+=("$(pluralize "$count" record records)")
  else
    parts+=("$(pluralize "$lines" line lines)")
  fi
  [[ -n "$version" ]]                       && parts+=("v$version")
  [[ "$truncated" == "true" ]]              && parts+=("TRUNCATED")
  [[ "$healthy" == "true" ]]                && parts+=("healthy")
  [[ "$healthy" == "false" ]]               && parts+=("unhealthy")
  [[ -n "$warnings" && "$warnings" != "" ]] && parts+=("warnings")
  parts+=("${bytes}B")

  local joined
  printf -v joined ', %s' "${parts[@]}"
  printf ' (%s)' "${joined:2}"
}

# Run a probe that must exit 0. All progress output goes to stderr so callers
# can capture command stdout if they need to.
probe() {
  local name="$1"; shift
  local rc
  $WSECTL "$@" >"$TMPDIR/out" 2>"$TMPDIR/err"
  rc=$?
  if [[ $rc -eq 0 ]]; then
    printf '\033[32m✓\033[0m %s%s\n' "$name" "$(probe_info "$TMPDIR/out")" >&2
    PASS=$((PASS+1))
    return 0
  fi
  printf '\033[31m✗\033[0m %s (exit %d)\n' "$name" "$rc" >&2
  sed 's/^/    /' < "$TMPDIR/err" >&2
  FAIL=$((FAIL+1))
  return 1
}

# Run a probe that must exit with a specific non-zero code.
expect_exit() {
  local name="$1" want="$2"; shift 2
  local rc
  $WSECTL "$@" >/dev/null 2>&1
  rc=$?
  if [[ $rc -eq $want ]]; then
    printf '\033[32m✓\033[0m %s (exit %d as expected)\n' "$name" "$want" >&2
    PASS=$((PASS+1))
  else
    printf '\033[31m✗\033[0m %s (got exit %d, want %d)\n' "$name" "$rc" "$want" >&2
    FAIL=$((FAIL+1))
  fi
}

probe_history() {
  local history_file="$TMPDIR/history-probe.jsonl"
  local config_file="$TMPDIR/history-config.toml"
  local secret_file="$TMPDIR/history-secret.json"
  cat >"$config_file" <<EOF
current_profile = "default"

[defaults]
output = "auto"
rate_limit = "1/s"
timeout = "30s"

[profiles.default]
account_url = "https://example.worksection.com"
auth_type = "oauth2"
secret_ref = "plaintext:$secret_file"
EOF

  local rc=0
  WSECTL_CONFIG="$config_file" WSECTL_HISTORY=1 WSECTL_HISTORY_FILE="$history_file" \
    $WSECTL auth login --manual-code --client-id history-probe --client-secret SECRET-VALUE \
    >"$TMPDIR/out" 2>"$TMPDIR/err" || rc=$?
  if [[ $rc -ne 0 ]]; then
    printf '\033[31m✗\033[0m history records redacted local command (exit %d)\n' "$rc" >&2
    sed 's/^/    /' < "$TMPDIR/err" >&2
    FAIL=$((FAIL+1))
    return
  fi
  if [[ ! -s "$history_file" ]]; then
    printf '\033[31m✗\033[0m history records redacted local command (no history file)\n' >&2
    FAIL=$((FAIL+1))
    return
  fi
  if grep -q 'SECRET-VALUE' "$history_file"; then
    printf '\033[31m✗\033[0m history records redacted local command (secret leaked)\n' >&2
    FAIL=$((FAIL+1))
    return
  fi
  if ! grep -q '\[redacted\]' "$history_file"; then
    printf '\033[31m✗\033[0m history records redacted local command (redaction marker missing)\n' >&2
    FAIL=$((FAIL+1))
    return
  fi
  printf '\033[32m✓\033[0m history records redacted local command\n' >&2
  PASS=$((PASS+1))
}

skip() {
  printf '\033[33m·\033[0m %s (skipped: %s)\n' "$1" "$2" >&2
  SKIP=$((SKIP+1))
}

# extract_id ACTION...  -> first .data[0].id, stripped of surrounding quotes.
# Returns empty string if the call fails or the field is null.
extract_id() {
  local raw
  raw="$($WSECTL "$@" --jq '.data[0].id' --json 2>/dev/null)" || return 0
  printf '%s' "$raw" | tr -d '"\n'
}

# extract_jq EXPR ACTION...  -> first scalar match for the given jq expression.
extract_jq() {
  local expr="$1"; shift
  local raw
  raw="$($WSECTL "$@" --jq "$expr" --json 2>/dev/null)" || return 0
  printf '%s' "$raw" | tr -d '"\n'
}

# file_in_task TASK_ID  -> first file ID attached to the task or any of its
# comments. Prints the ID, prints a diagnostic of what was checked to stderr.
file_in_task() {
  local task="$1" fid
  fid="$(extract_id files list --task "$task")"
  if [[ -n "$fid" && "$fid" != "null" ]]; then
    printf '    task %s: task-attached file %s\n' "$task" "$fid" >&2
    printf '%s' "$fid"; return 0
  fi
  fid="$(extract_jq '[.data[].files[]?.id][0]' comments list "$task" --extra files)"
  if [[ -n "$fid" && "$fid" != "null" ]]; then
    printf '    task %s: comment-attached file %s\n' "$task" "$fid" >&2
    printf '%s' "$fid"; return 0
  fi
  printf '    task %s: no file in task or comments\n' "$task" >&2
}

# discover_file_id PROJECT_ID  -> walks the first few tasks in the project.
discover_file_id() {
  local project="$1"
  local task_ids fid task
  task_ids="$($WSECTL tasks list --project "$project" --jq '[.data[].id][:5][]' --json 2>/dev/null | tr -d '"')" || return 0
  for task in $task_ids; do
    fid="$(file_in_task "$task")"
    if [[ -n "$fid" && "$fid" != "null" ]]; then
      printf '%s' "$fid"; return 0
    fi
  done
}

echo "wsectl live probe"
echo "Account:  ${WSECTL_ACCOUNT_URL:-from active profile}"
echo "Binary:   $WSECTL"
echo

#
# 1. Health & identity
#
probe "doctor --api --json" doctor --api --json || true
probe "me --json"           me --json           || true

#
# 2. Discovery surface
#
probe "commands --json"    commands --json    || true
probe "api actions --json" api actions --json || true
probe "version --json"     version --json     || true
probe_history

#
# 3. Projects path (self-bootstrap PROJECT_ID)
#
probe "projects list --json" projects list --json || true
PROJECT_ID="${WSECTL_PROBE_PROJECT:-$(extract_id projects list)}"

if [[ -n "$PROJECT_ID" && "$PROJECT_ID" != "null" ]]; then
  probe "projects get $PROJECT_ID --extra text,users --json" \
    projects get "$PROJECT_ID" --extra text,users --json || true
else
  skip "projects get <id>" "no project discovered"
fi

#
# 4. Tasks path (self-bootstrap TASK_ID)
#
TASK_ID="${WSECTL_PROBE_TASK:-}"
if [[ -n "$PROJECT_ID" && "$PROJECT_ID" != "null" ]]; then
  probe "tasks list --project $PROJECT_ID --json" \
    tasks list --project "$PROJECT_ID" --json || true
  if [[ -z "$TASK_ID" ]]; then
    TASK_ID="$(extract_id tasks list --project "$PROJECT_ID")"
  fi
else
  skip "tasks list --project" "no project discovered"
fi

if [[ -n "$TASK_ID" && "$TASK_ID" != "null" ]]; then
  probe "tasks get $TASK_ID --extra files,comments --json" \
    tasks get "$TASK_ID" --extra files,comments --json || true
  probe "comments list $TASK_ID --json" \
    comments list "$TASK_ID" --json || true
else
  skip "tasks get / comments list" "no task discovered"
fi

probe "tasks search --query test --json" \
  tasks search --query "test" --json || true

#
# 5. File download path (S1 verification: same-host bearer attachment)
#
FILE_ID="${WSECTL_PROBE_FILE:-}"
if [[ -z "$FILE_ID" && -n "$TASK_ID" && "$TASK_ID" != "null" ]]; then
  probe "files list --task $TASK_ID --json" \
    files list --task "$TASK_ID" --json || true
  printf '  searching pinned task %s for any file ...\n' "$TASK_ID" >&2
  FILE_ID="$(file_in_task "$TASK_ID")"
fi

if [[ -z "$FILE_ID" && -n "$PROJECT_ID" && "$PROJECT_ID" != "null" ]]; then
  printf '  searching first 5 tasks of project %s ...\n' "$PROJECT_ID" >&2
  FILE_ID="$(discover_file_id "$PROJECT_ID")"
fi

if [[ -n "$FILE_ID" && "$FILE_ID" != "null" ]]; then
  DOWNLOAD_PATH="$TMPDIR/probe-download.bin"
  rc=0
  $WSECTL files download "$FILE_ID" --out "$DOWNLOAD_PATH" >/dev/null 2>"$TMPDIR/err" || rc=$?
  if [[ $rc -ne 0 ]]; then
    printf '\033[31m✗\033[0m files download %s (exit %d)\n' "$FILE_ID" "$rc" >&2
    sed 's/^/    /' < "$TMPDIR/err" >&2
    FAIL=$((FAIL+1))
  elif [[ ! -s "$DOWNLOAD_PATH" ]]; then
    printf '\033[31m✗\033[0m files download %s produced an empty file\n' "$FILE_ID" >&2
    FAIL=$((FAIL+1))
  else
    SIZE="$(wc -c < "$DOWNLOAD_PATH" | tr -d ' ')"
    printf '\033[32m✓\033[0m files download %s (%sB to %s)\n' "$FILE_ID" "$SIZE" "$DOWNLOAD_PATH" >&2
    PASS=$((PASS+1))
  fi
else
  skip "files download" "no file found in the first 5 tasks (set WSECTL_PROBE_FILE to pin one)"
fi

#
# 6. Output-format and contract surface
#
probe "users list --ndjson"           users list --ndjson           || true
probe "users list --table"            users list --table            || true
probe "projects list --schema --json" projects list --schema --json || true
probe "projects list --json --fields id,name --limit 3" \
  projects list --json --fields id,name --limit 3 || true

#
# 7. Low-level api call
#
probe "api call get_users --json" api call get_users --json || true

#
# 8. Negative cases (exit-code contract)
#
expect_exit "unknown action -> exit 2"         2 api call no_such_action --json
expect_exit "missing required param -> exit 2" 2 api call get_project    --json

#
# Summary
#
echo >&2
printf 'passed=%d failed=%d skipped=%d\n' "$PASS" "$FAIL" "$SKIP" >&2
[[ $FAIL -eq 0 ]]
