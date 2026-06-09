#!/usr/bin/env bash
# Apply or update GitHub repository rulesets from .github/rulesets/*.json.
#
# Requires: gh CLI authenticated against the target repo, jq.
#
# Usage:
#   scripts/apply-rulesets.sh                  # apply to pbv7/wsectl
#   REPO=owner/name scripts/apply-rulesets.sh  # apply to a different repo
#   DRY_RUN=1 scripts/apply-rulesets.sh        # print intended actions; no API calls, no auth needed
#
# Idempotent: ruleset matched by .name; PUT if it exists, POST otherwise.
# Also enables repo-level delete_branch_on_merge.

set -euo pipefail

REPO="${REPO:-pbv7/wsectl}"
DRY_RUN="${DRY_RUN:-}"
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
RULESETS_DIR="$ROOT_DIR/.github/rulesets"

if [ -z "$DRY_RUN" ] && ! command -v gh >/dev/null 2>&1; then
  echo "error: gh CLI not found in PATH" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq not found in PATH" >&2
  exit 1
fi

apply_ruleset() {
  local file="$1"
  local name
  name=$(jq -r '.name' "$file")
  if [ -z "$name" ] || [ "$name" = "null" ]; then
    echo "error: $file has no .name field" >&2
    exit 1
  fi

  if [ -n "$DRY_RUN" ]; then
    echo "[dry-run] would look up ruleset '$name' on $REPO via GET repos/$REPO/rulesets"
    echo "[dry-run]   if it exists: PUT repos/$REPO/rulesets/<id> --input $file"
    echo "[dry-run]   if it does not exist: POST repos/$REPO/rulesets --input $file"
    return 0
  fi

  local existing_id
  existing_id=$(gh api "repos/$REPO/rulesets" --jq ".[] | select(.name == \"$name\") | .id" 2>/dev/null || true)

  if [ -n "$existing_id" ]; then
    echo "Updating ruleset '$name' (id=$existing_id) on $REPO"
    gh api -X PUT "repos/$REPO/rulesets/$existing_id" --input "$file" >/dev/null
  else
    echo "Creating ruleset '$name' on $REPO"
    gh api -X POST "repos/$REPO/rulesets" --input "$file" >/dev/null
  fi
}

shopt -s nullglob
for f in "$RULESETS_DIR"/*.json; do
  apply_ruleset "$f"
done
shopt -u nullglob

if [ -n "$DRY_RUN" ]; then
  echo "[dry-run] would PATCH repos/$REPO with delete_branch_on_merge=true"
else
  echo "Enabling delete_branch_on_merge on $REPO"
  gh api -X PATCH "repos/$REPO" -F delete_branch_on_merge=true >/dev/null
fi

echo "Done."
