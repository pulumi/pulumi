#!/usr/bin/env bash
# Backfill PR numbers in changie fragments from git history.
#
# For each pending changie fragment that has an empty PR field,
# find the commit that introduced the file and extract the PR
# number from the merge commit message (squash-merge pattern: "... (#NNNNN)").
#
# Requires: yq (https://github.com/mikefarah/yq)

set -euo pipefail

PENDING_DIR="changelog/pending"

if [ ! -d "${PENDING_DIR}" ]; then
  echo "No pending directory found." >&2
  exit 0
fi

for f in "${PENDING_DIR}"/*.yaml; do
  [ -f "$f" ] || continue

  PR=$(yq -r '.custom.PR // ""' "$f")
  if [ -n "$PR" ]; then
    continue
  fi

  # Find the commit that added this file
  COMMIT=$(git log --diff-filter=A --format='%H' -- "$f" 2>/dev/null | head -1)
  if [ -z "$COMMIT" ]; then
    continue
  fi

  # Extract PR number from commit message (#NNNNN)
  PR_NUM=$(git log --format='%s' -1 "$COMMIT" | grep -oE '#[0-9]+' | head -1 | tr -d '#')
  if [ -z "$PR_NUM" ]; then
    continue
  fi

  yq -i ".custom.PR = \"$PR_NUM\"" "$f"
  echo "Backfilled PR #${PR_NUM} in $(basename "$f")"
done
