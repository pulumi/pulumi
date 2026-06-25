#!/usr/bin/env bash

set -eo pipefail
set -x

LOCAL="${1:-"false"}"

# Use github credentials for a higher rate limit when possible.
USE_GH=false
if command -v gh >/dev/null && gh auth status >/dev/null 2>&1; then
  USE_GH=true
fi

retry_with_backoff() {
    local max_attempts=3
    local attempt=1
    local exitcode=0

    while [ ${attempt} -le ${max_attempts} ]; do
        if "$@"; then
            return 0
        fi

        exitcode=$?

        if [ ${attempt} -lt ${max_attempts} ]; then
            local backoff=$((2 ** (attempt - 1)))
            sleep ${backoff}
        fi

        attempt=$((attempt + 1))
    done

    return ${exitcode}
}

download_release() {
  local owner="$1"
  local lang="$2"
  local tag="$3"
  local filename="$4"

  if "${USE_GH}"; then
    retry_with_backoff gh release download "${tag}" --repo "${owner}/pulumi-${lang}" -p "${filename}"
  else
    curl -OL --fail --retry 3 "https://github.com/${owner}/pulumi-${lang}/releases/download/${tag}/${filename}"
  fi
}

# The pinned language-runtime versions live in versions.json at the repo root (the single
# source of truth, kept up to date by renovate). We read the bundled runtimes from there into
# the LANGUAGES array, each entry as "lang version owner" where owner is the GitHub org from
# the runtime's repo field.
#
# Note: the HCL language runtime is no longer bundled. Its pinned version and download URL
# live in pkg/util/plugin.go (knownLanguageRuntimes) and the CLI fetches it on demand, so it is
# intentionally absent from the bundled list below.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSIONS_JSON="${SCRIPT_DIR}/../versions.json"

if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq is required to read ${VERSIONS_JSON}" >&2
  exit 1
fi

# Capture jq's output first so a failure aborts under 'set -e'.
LANGUAGES_TSV="$(jq -r '.bundledLanguageRuntimes | to_entries[] | "\(.key) \(.value.version) \(.value.repo | split("/")[0])"' "${VERSIONS_JSON}")"
LANGUAGES=()
while IFS= read -r entry; do
  [ -n "${entry}" ] && LANGUAGES+=("${entry}")
done <<< "${LANGUAGES_TSV}"

for i in "${LANGUAGES[@]}"; do
  set -- $i # treat strings in loop as args
  PULUMI_LANG="$1"
  TAG="$2"
  PULUMI_OWNER="${3:-pulumi}"

  LANG_DIST="$(pwd)/bin"
  mkdir -p "${LANG_DIST}"
  (
    # Run in a subshell to ensure we don't alter current working directory.
    cd "$(mktemp -d)"

    # Currently avoiding a dependency on GH CLI in favor of curl, so
    # that this script works in the context of the Brew formula:
    #
    # https://github.com/Homebrew/homebrew-core/blob/master/Formula/pulumi.rb
    #
    # Formerly:
    #
    # gh release download "${TAG}" --repo "pulumi/pulumi-${PULUMI_LANG}"

    for j in "darwin" "linux" "windows .exe"; do
      set -- $j # treat strings in loop as args
      DIST_OS="$1"
      DIST_EXT="${2:-""}"

      for k in "amd64 x64" "arm64 arm64"; do
        set -- $k # treat strings in loop as args
        DIST_ARCH="$1"
        RENAMED_ARCH="$2" # goreleaser in pulumi/pulumi renames amd64 to x64

        # if TARGET is set and DIST_OS-DIST_ARCH does not match, skip
        if [ "${LOCAL}" = "local" ] && [ "$(go env GOOS)-$(go env GOARCH)" != "${DIST_OS}-${DIST_ARCH}" ]; then
            continue
        fi

        ARCHIVE="pulumi-language-${PULUMI_LANG}-${TAG}-${DIST_OS}-${DIST_ARCH}"

        OUTDIR="${LANG_DIST}/$DIST_OS-$RENAMED_ARCH"

        mkdir -p "${OUTDIR}"

        download_release "${PULUMI_OWNER}" "${PULUMI_LANG}" "${TAG}" "${ARCHIVE}.tar.gz"
        tar -xzvf "${ARCHIVE}.tar.gz" -C "${OUTDIR}" "pulumi-language-${PULUMI_LANG}${DIST_EXT}"
      done
    done
  )
done
