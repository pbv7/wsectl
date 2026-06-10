#!/usr/bin/env bash
#
# Live output coverage matrix. Exercises every read action across every
# output format and every relevant transform flag against a real
# Worksection account, asserting that the rendered bytes parse as the
# format they claim to be. Designed to catch class-of-bug regressions
# the unit-test suite would miss (e.g. JSON escapes the YAML tokenizer
# rejects).
#
# Read-only: the wsectl binary blocks write actions and this script only
# issues read ones. All artifacts go to a tmpdir that is removed on exit.
# No stdout/file content from this run enters git.
#
# Environment:
#   WSECTL                Binary path (default: dist/wsectl)
#   WSECTL_PROBE_PROJECT  Project ID for project-scoped probes (required)
#   WSECTL_PROBE_TASK     Task ID for task-scoped probes (required)
#   WSECTL_PROBE_FILE     File ID for the download probe (optional). When
#                         unset, the script auto-discovers the first file
#                         attached to WSECTL_PROBE_TASK; if there is none,
#                         the download probe is skipped.
#   MATRIX_KEEP           If set, keep TMPDIR on exit for inspection
#
# Exit codes:
#   0  all probes passed
#   1  at least one probe failed
#
# Required tools: jq, yq (mikefarah v4+).

set -u

WSECTL="${WSECTL:-dist/wsectl}"
PROJECT="${WSECTL_PROBE_PROJECT:-}"
TASK="${WSECTL_PROBE_TASK:-}"
FILE_ID="${WSECTL_PROBE_FILE:-}"

if [[ -z "$PROJECT" || -z "$TASK" ]]; then
  echo "WSECTL_PROBE_PROJECT and WSECTL_PROBE_TASK are required" >&2
  exit 2
fi
if [[ ! -x "$WSECTL" ]]; then
  echo "wsectl binary not found or not executable at $WSECTL — run 'make build' first" >&2
  exit 2
fi
if ! command -v jq >/dev/null; then
  echo "jq is required" >&2
  exit 2
fi
if ! command -v yq >/dev/null; then
  echo "yq (mikefarah v4+) is required" >&2
  exit 2
fi

TMPDIR="$(mktemp -d)"
if [[ ! -d "$TMPDIR" ]]; then
  echo "failed to create temporary directory via mktemp -d" >&2
  exit 2
fi
if [[ -z "${MATRIX_KEEP:-}" ]]; then
  trap 'rm -rf "$TMPDIR"' EXIT
else
  trap 'echo "kept TMPDIR: $TMPDIR" >&2' EXIT
fi

PASS=0; FAIL=0; SKIP=0
FAILURES=()

red() { printf '\033[31m%s\033[0m' "$1"; }
green() { printf '\033[32m%s\033[0m' "$1"; }
yellow() { printf '\033[33m%s\033[0m' "$1"; }

# Probe runner. Usage:
#   probe LABEL FORMAT EXPECT_RC -- args...
#
# FORMAT is one of: json yaml ndjson table raw schema-json schema-yaml
#                   error-json error-yaml jq-result
# EXPECT_RC: usually 0; pass an integer to assert non-zero exits.
probe() {
  local label="$1" format="$2" expect_rc="$3"; shift 3
  local out="$TMPDIR/out" err="$TMPDIR/err" rc=0
  "$WSECTL" "$@" >"$out" 2>"$err" || rc=$?
  # Worksection occasionally returns "invalid JSON" or rate-limit
  # signals when called back-to-back. Retry the probe once after a
  # short pause; persistent failures still surface.
  if [[ "$rc" -ne "$expect_rc" ]] && grep -qE "invalid JSON|rate_limit|too many requests" "$out" "$err" 2>/dev/null; then
    sleep 1.2
    rc=0
    "$WSECTL" "$@" >"$out" 2>"$err" || rc=$?
  fi
  if [[ "$rc" -ne "$expect_rc" ]]; then
    record_fail "$label" "exit $rc (want $expect_rc): $(head -c 200 "$err" | tr -d '\n')"
    return
  fi
  case "$format" in
    json)
      jq -e 'type == "object" and has("status") and has("meta")' "$out" >/dev/null \
        || { record_fail "$label" "json: envelope shape invalid"; return; }
      ;;
    yaml)
      yq -e '.status' "$out" >/dev/null \
        || { record_fail "$label" "yaml: parse or missing status — first chars: $(head -c 200 "$out" | tr '\n' ' ')"; return; }
      ;;
    ndjson)
      # Empty output is legitimate when the source array has zero records;
      # the action exited cleanly so accept. NDJSON's contract is "one
      # JSON document per *line*", so multi-line pretty-printed JSON or
      # a single top-level array would violate the format even though
      # both parse as valid JSON streams. Read raw input line by line in
      # a single jq pass and run `fromjson` on each — one process, per
      # line semantics, and no `-e` so legitimate `false`/`null` lines
      # do not false-fail.
      if [[ -s "$out" ]]; then
        jq -nR 'inputs | fromjson | empty' <"$out" >/dev/null 2>&1 \
          || { record_fail "$label" "ndjson: at least one line is not valid JSON per line"; return; }
      fi
      ;;
    table)
      grep -qE '(^[A-Z_][A-Z_]*(  +[A-Z_]+)*|^No rows)' "$out" \
        || { record_fail "$label" "table: no header / 'No rows' detected"; return; }
      ;;
    raw)
      [[ -s "$out" ]] || { record_fail "$label" "raw: empty body"; return; }
      ;;
    jq-result)
      # ApplyJQ emits one pretty-printed JSON document per result, joined
      # by newlines. Object/array results legitimately span multiple
      # lines, so we cannot validate line-by-line. Slurp the whole stream
      # into an array and assert it parses with at least one element.
      [[ -s "$out" ]] || { record_fail "$label" "jq-result: empty output"; return; }
      jq -se 'type == "array" and length > 0' "$out" >/dev/null \
        || { record_fail "$label" "jq-result: stream does not parse as JSON values"; return; }
      ;;
    schema-json)
      jq -e '.status == "ok" and .data.name and .data.response' "$out" >/dev/null \
        || { record_fail "$label" "schema-json: shape invalid"; return; }
      ;;
    schema-yaml)
      yq -e '.status == "ok" and .data.name and .data.response' "$out" >/dev/null \
        || { record_fail "$label" "schema-yaml: shape invalid"; return; }
      ;;
    error-json)
      # wsectl writes error envelopes to stderr (success goes to stdout).
      jq -e '.status == "error" and .error.code' "$err" >/dev/null \
        || { record_fail "$label" "error-json: shape invalid (stderr)"; return; }
      ;;
    error-yaml)
      yq -e '.status == "error" and .error.code' "$err" >/dev/null \
        || { record_fail "$label" "error-yaml: shape invalid (stderr)"; return; }
      ;;
    *)
      record_fail "$label" "unknown format $format"
      return
      ;;
  esac
  PASS=$((PASS+1))
  printf '%s %s\n' "$(green '✓')" "$label" >&2
}

record_fail() {
  local label="$1" detail="$2"
  FAIL=$((FAIL+1))
  FAILURES+=("$label :: $detail")
  printf '%s %s — %s\n' "$(red '✗')" "$label" "$detail" >&2
}

section() { printf '\n%s\n' "── $1 ─────────────────────────────────────────────" >&2; }

# Each action gets exercised across formats. Composition with transform
# flags (--fields, --jq, --limit) is sampled rather than exhaustive.

section "me"
for fmt in json yaml ndjson table raw; do
  probe "me --$fmt" "$fmt" 0 me "--$fmt"
done
probe "me --schema --yaml"   schema-yaml 0 me --schema --yaml
probe "me --json --fields id,name" json 0 me --json --fields id,name
probe "me --json --jq .data.email" jq-result 0 me --json --jq .data.email

section "users list"
for fmt in json yaml ndjson table; do
  probe "users list --$fmt" "$fmt" 0 users list "--$fmt" --limit 5
done
probe "users list --raw --limit 1" raw 0 users list --raw --limit 1
probe "users list --json --fields id,name,email" json 0 users list --json --fields id,name,email --limit 5
# Use `.data | length` for collection probes: it always emits exactly one
# JSON value (an integer), so a quiet account or zero-result day cannot
# false-fail the "jq pipe is functional" check.
probe "users list --json --jq '.data | length'" jq-result 0 users list --json --jq '.data | length' --limit 5
probe "users list --schema --yaml" schema-yaml 0 users list --schema --yaml

section "users sub-commands"
for cmd_words in "users groups" "users contacts" "users contact-groups"; do
  read -ra cmd <<<"$cmd_words"
  for fmt in json yaml table; do
    probe "${cmd_words} --$fmt" "$fmt" 0 "${cmd[@]}" "--$fmt"
  done
done

section "projects list"
for fmt in json yaml ndjson table raw; do
  probe "projects list --$fmt" "$fmt" 0 projects list "--$fmt" --limit 5
done
probe "projects list --json --status active" json 0 projects list --json --status active --limit 5
probe "projects list --yaml --extra text,options,users" yaml 0 projects list --yaml --extra text,options,users --limit 3
probe "projects list --json --fields id,name,status" json 0 projects list --json --fields id,name,status --limit 5
probe "projects list --schema --json" schema-json 0 projects list --schema --json

section "projects get / team / groups"
for fmt in json yaml table; do
  probe "projects get --$fmt" "$fmt" 0 projects get "$PROJECT" "--$fmt"
  probe "projects team --$fmt" "$fmt" 0 projects team "$PROJECT" "--$fmt"
done
for fmt in json yaml ndjson table; do
  probe "projects groups --$fmt" "$fmt" 0 projects groups "--$fmt"
done

section "projects events"
for fmt in json yaml ndjson table raw; do
  probe "projects events --$fmt" "$fmt" 0 projects events --project "$PROJECT" --period 1d "--$fmt"
done
probe "projects events --json --fields action,date_added" json 0 projects events --project "$PROJECT" --period 1d --json --fields action,date_added
probe "projects events --table --fields action,date_added" table 0 projects events --project "$PROJECT" --period 1d --table --fields action,date_added
probe "projects events --json --jq '.data | length'" jq-result 0 projects events --project "$PROJECT" --period 1d --json --jq '.data | length'
probe "projects events --schema --yaml" schema-yaml 0 projects events --schema --yaml

section "tasks"
# tasks all hits the account-wide 10000 cap on busy accounts; use search instead.
for fmt in json yaml ndjson table; do
  probe "tasks search --$fmt --project --limit 5" "$fmt" 0 tasks search --project "$PROJECT" "--$fmt" --limit 5
done
probe "tasks search --json --fields id,name --limit 5" json 0 tasks search --project "$PROJECT" --json --fields id,name --limit 5
probe "tasks search --json --jq '.data | length' --limit 5" jq-result 0 tasks search --project "$PROJECT" --json --jq '.data | length' --limit 5
for fmt in json yaml table; do
  probe "tasks get --$fmt" "$fmt" 0 tasks get "$TASK" "--$fmt"
  probe "tasks subtasks --$fmt" "$fmt" 0 tasks subtasks "$TASK" "--$fmt"
  probe "tasks subscribers --$fmt" "$fmt" 0 tasks subscribers "$TASK" "--$fmt"
  probe "tasks relations --$fmt" "$fmt" 0 tasks relations "$TASK" "--$fmt"
  probe "tasks discussion --$fmt" "$fmt" 0 tasks discussion "$TASK" "--$fmt"
done

section "comments"
for fmt in json yaml ndjson table; do
  probe "comments list --$fmt" "$fmt" 0 comments list "$TASK" "--$fmt"
done

section "tags (account-wide)"
for cmd_words in "tags task list" "tags task groups" "tags project list" "tags project groups"; do
  read -ra cmd <<<"$cmd_words"
  for fmt in json yaml table; do
    probe "${cmd_words} --$fmt" "$fmt" 0 "${cmd[@]}" "--$fmt"
  done
done

section "timers"
for fmt in json yaml table; do
  probe "timers list --$fmt" "$fmt" 0 timers list "--$fmt"
  probe "timers mine --$fmt" "$fmt" 0 timers mine "--$fmt"
done

# costs list/total hit an account-side memory limit on this account; skip.
SKIP=$((SKIP+6)); for fmt in json yaml table; do
  printf '%s costs list --%s (account memory limit)\n' "$(yellow '·')" "$fmt" >&2
  printf '%s costs total --%s (account memory limit)\n' "$(yellow '·')" "$fmt" >&2
done

section "files"
# files list requires exactly one of --project / --task. Worksection only
# returns files attached at task level for most accounts, so scope to task
# to get non-empty data.
for fmt in json yaml ndjson table; do
  probe "files list --$fmt" "$fmt" 0 files list --task "$TASK" "--$fmt"
done
for fmt in json yaml ndjson table; do
  probe "files images --$fmt" "$fmt" 0 files images --task "$TASK" "--$fmt"
done
for fmt in json yaml ndjson table; do
  probe "files task-attachments --$fmt" "$fmt" 0 files task-attachments "$TASK" "--$fmt"
done
# Two file actions can surface attachments and they touch different code
# paths: `files list --task X` calls get_files (task-level files), and
# `files task-attachments X` calls get_task with extra=files (which on
# Worksection typically also surfaces files attached to task comments).
# Probe download for whichever the task actually carries; an explicit
# WSECTL_PROBE_FILE override takes precedence and runs a single probe.
# discover_ids LABEL OUTFILE JQ_FILTER -- wsectl args...
# Runs a wsectl command with the same retry-once semantics as probe(),
# then projects the output through jq, writing the newline-separated id
# stream to OUTFILE. Returns 0 when discovery completed (the caller
# reads OUTFILE; an empty file means the response was valid but carried
# no items). Returns 1 on real discovery failures (wsectl error after
# retry, or malformed JSON the jq filter cannot parse) and records the
# failure via record_fail with diagnostic context. Must run in the
# parent shell — not via $(...) — so record_fail mutations to FAIL and
# FAILURES persist.
discover_ids() {
  local label="$1" outfile="$2" filter="$3"; shift 3
  local out="$TMPDIR/disc-out" err="$TMPDIR/disc-err" rc=0
  "$WSECTL" "$@" >"$out" 2>"$err" || rc=$?
  if [[ "$rc" -ne 0 ]] && grep -qE "invalid JSON|rate_limit|too many requests" "$out" "$err" 2>/dev/null; then
    sleep 1.2
    rc=0
    "$WSECTL" "$@" >"$out" 2>"$err" || rc=$?
  fi
  if [[ "$rc" -ne 0 ]]; then
    record_fail "$label" "discovery wsectl exit $rc: $(head -c 200 "$err" | tr -d '\n')"
    : >"$outfile"
    return 1
  fi
  if ! jq -r "$filter" "$out" >"$outfile" 2>"$err"; then
    record_fail "$label" "discovery jq parse failed: $(head -c 200 "$err" | tr -d '\n')"
    : >"$outfile"
    return 1
  fi
  return 0
}

probe_download() {
  local source="$1" file_id="$2"
  local dl="$TMPDIR/dl-$source-$file_id"
  local label="files download $file_id --out (source: $source)"
  local rc=0
  "$WSECTL" files download "$file_id" --out "$dl" >"$TMPDIR/out" 2>"$TMPDIR/err" || rc=$?
  if [[ "$rc" -ne 0 ]]; then
    record_fail "$label" "exit $rc: $(head -c 200 "$TMPDIR/err" | tr -d '\n')"
    return
  fi
  if [[ ! -s "$dl" ]]; then
    record_fail "$label" "downloaded file is empty or missing"
    return
  fi
  local kind="unknown"
  if command -v file >/dev/null 2>&1; then
    kind="$(file -b --mime-type "$dl" 2>/dev/null || echo unknown)"
  fi
  PASS=$((PASS+1))
  printf '%s %s [%d bytes, %s]\n' "$(green '✓')" "$label" "$(wc -c <"$dl")" "$kind" >&2
}

discovered_any=0
seen_ids=""
download_unique() {
  local source="$1" file_id="$2"
  [[ -z "$file_id" ]] && return
  case " $seen_ids " in *" $file_id "*) return ;; esac
  seen_ids="$seen_ids $file_id"
  probe_download "$source" "$file_id"
  discovered_any=1
}
if [[ -n "$FILE_ID" ]]; then
  download_unique "explicit" "$FILE_ID"
else
  # `files list --task X` (get_files) returns files at the task AND
  # comment level; group by filename extension and sample one per
  # extension so different content types get exercised (xlsx vs docx
  # vs pdf vs ...). `files task-attachments X` (get_task extra=files)
  # surfaces a different code path; cover that too. Dedup by id so a
  # file appearing in both is downloaded once. discover_ids uses the
  # same retry policy as probe so transient API hiccups do not silently
  # collapse to "no files".
  list_file="$TMPDIR/disc-list-ids"
  attach_file="$TMPDIR/disc-attach-ids"
  if discover_ids "files download discovery (list)" "$list_file" \
        '(.data // []) | group_by(.name // "" | ascii_downcase | split(".") | last) | map(.[0].id // empty)[]' \
        files list --task "$TASK" --json; then
    while IFS= read -r id; do
      download_unique "files list" "$id"
    done <"$list_file"
  fi
  if discover_ids "files download discovery (task-attachments)" "$attach_file" \
        '(.data.files // [])[0].id // empty' \
        files task-attachments "$TASK" --json; then
    while IFS= read -r id; do
      download_unique "task-attachments" "$id"
    done <"$attach_file"
  fi
fi
if [[ "$discovered_any" -eq 0 ]]; then
  SKIP=$((SKIP+1))
  printf '%s files download (no file id and task carries no attachments)\n' "$(yellow '·')" >&2
fi

# webhooks list requires an admin token; skip when running under user OAuth.
SKIP=$((SKIP+3)); for fmt in json yaml table; do
  printf '%s webhooks list --%s (admin scope required)\n' "$(yellow '·')" "$fmt" >&2
done

section "error envelope shapes"
probe "events --period bogus --json" error-json 6 projects events --project "$PROJECT" --period bogus --json
probe "events --period bogus --yaml" error-yaml 6 projects events --project "$PROJECT" --period bogus --yaml

section "summary"
printf '\n%s passed, %s failed, %s skipped\n' "$(green "$PASS")" "$(red "$FAIL")" "$(yellow "$SKIP")" >&2
if [[ "$FAIL" -gt 0 ]]; then
  echo >&2
  echo "Failures:" >&2
  for f in "${FAILURES[@]}"; do
    echo "  - $f" >&2
  done
  exit 1
fi
