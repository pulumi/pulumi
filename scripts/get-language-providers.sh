#!/usr/bin/env bash

set -eo pipefail
set -x

LOCAL="${1:-"false"}"

# Use github credentials for a higher rate limit when possible.
USE_GH=false
if command -v gh >/dev/null && gh auth status >/dev/null 2>&1; then
  USE_GH=true
fi

download_release() {
  local lang="$1"
  local tag="$2"
  local filename="$3"

  if "${USE_GH}"; then
    gh release download "${tag}" --repo "pulumi/pulumi-${lang}" -p "${filename}"
  else
    curl -OL --fail --retry 3 "https://github.com/pulumi/pulumi-${lang}/releases/download/${tag}/${filename}"
  fi
}

LANGUAGES=(
  "dotnet v3.101.2"
  "java v1.21.2"
  "yaml v1.29.1"
)

for i in "${LANGUAGES[@]}"; do
  set -- $i # treat strings in loop as args
  PULUMI_LANG="$1"
  TAG="$2"

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

        download_release "${PULUMI_LANG}" "${TAG}" "${ARCHIVE}.tar.gz"
        tar -xzvf "${ARCHIVE}.tar.gz" -C "${OUTDIR}" "pulumi-language-${PULUMI_LANG}${DIST_EXT}"
      done
    done
  )
done
