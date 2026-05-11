#!/usr/bin/env bash
# Download every package schema from the Pulumi registry and run
# `pulumi schema check` against each one. Used to validate that bind-time
# changes (e.g. the unconstructable-types check) don't break any existing
# published schema.
#
# Requires:
#   - the locally built pulumi at $REPO/bin/pulumi (so we use the version
#     under test, not whatever is on $PATH)
#   - jq, curl
#   - logged in to Pulumi Cloud (`pulumi login`)
#
# Output: each line is one of OK / FAIL / DOWNLOAD_FAILED / SKIP, followed
# by the package id. On FAIL the schema-check stderr is printed inline.
# A summary is printed at the end. All artifacts (downloaded schemas and
# per-package error logs) are kept in $WORK_DIR.

set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PULUMI_BIN="${PULUMI_BIN:-$REPO/bin/pulumi}"
WORK_DIR="${WORK_DIR:-$(mktemp -d -t pulumi-registry-check.XXXXXX)}"

if [[ ! -x "$PULUMI_BIN" ]]; then
  echo "pulumi binary not found at $PULUMI_BIN; build it with 'make build' or set PULUMI_BIN" >&2
  exit 2
fi

mkdir -p "$WORK_DIR/schemas" "$WORK_DIR/errors"

echo "pulumi:    $PULUMI_BIN"
echo "work dir:  $WORK_DIR"

make -C "$REPO" bin/pulumi

# --- 1. Pull the package list. -----------------------------------------------
# ListPackages is paginated (100 per page); --paginate follows the
# continuationToken automatically and merges pages.
PACKAGES_JSON="$WORK_DIR/packages.json"
echo "Listing packages..."
"$PULUMI_BIN" cloud api ListPackages --paginate >"$PACKAGES_JSON"
TOTAL=$(jq '.packages | length' "$PACKAGES_JSON")
echo "Found $TOTAL packages."

# --- 2. Walk each package: download schema + run schema check. ---------------
# schemaURL is a presigned S3 link with a short expiry (~5 minutes), so we
# download and check each package back-to-back rather than batching.
PASS=0
FAIL=0
SKIP=0
DLERR=0
FAIL_IDS=()

# Read packages into a bash array so the counters in the loop survive
# (a piped `while read` would run in a subshell). `mapfile` is bash 4+, so
# stash NUL-delimited records in a tmp file and iterate that — works on
# macOS's default bash 3.2 too.
PKG_LIST="$WORK_DIR/packages.ndjson"
jq -c '.packages[]' "$PACKAGES_JSON" >"$PKG_LIST"

i=0
while IFS= read -r pkg; do
  i=$((i + 1))
  source=$(jq -r '.source' <<<"$pkg")
  publisher=$(jq -r '.publisher' <<<"$pkg")
  name=$(jq -r '.name' <<<"$pkg")
  version=$(jq -r '.version' <<<"$pkg")
  schema_url=$(jq -r '.schemaURL // ""' <<<"$pkg")

  id="${source}__${publisher}__${name}__${version}"
  schema_file="$WORK_DIR/schemas/${id}.json"
  err_file="$WORK_DIR/errors/${id}.txt"
  prefix="[$i/$TOTAL]"

  if [[ -z "$schema_url" ]]; then
    echo "$prefix SKIP (no schemaURL): $id"
    SKIP=$((SKIP + 1))
    continue
  fi

  # The registry serves schemas with Content-Encoding: gzip unconditionally
  # (see scripts/registry-content-encoding-bug.md), so we always send
  # Accept-Encoding: gzip via --compressed and let curl decompress.
  if ! curl -sSfL --compressed "$schema_url" -o "$schema_file" 2>"$err_file"; then
    echo "$prefix DOWNLOAD_FAILED: $id"
    cat "$err_file"
    DLERR=$((DLERR + 1))
    continue
  fi

  if "$PULUMI_BIN" schema check --allow-dangling-references "$schema_file" >"$err_file" 2>&1; then
    echo "$prefix OK: $id"
    PASS=$((PASS + 1))
  else
    echo "$prefix FAIL: $id"
    sed 's/^/    /' "$err_file"
    FAIL=$((FAIL + 1))
    FAIL_IDS+=("$id")
  fi
done <"$PKG_LIST"

echo
echo "==================== summary ===================="
echo "total:           $TOTAL"
echo "passed:          $PASS"
echo "failed:          $FAIL"
echo "skipped:         $SKIP"
echo "download errors: $DLERR"
if (( FAIL > 0 )); then
  echo
  echo "Failed packages:"
  for id in "${FAIL_IDS[@]}"; do
    echo "  - $id"
  done
fi
echo
echo "Artifacts kept in: $WORK_DIR"
