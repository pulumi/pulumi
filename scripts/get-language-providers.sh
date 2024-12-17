#!/usr/bin/env bash

set -eo pipefail
set -x

LOCAL="${1:-"false"}"

get_version() {
  local repo="$1"
  (
    cd pkg
    GOWORK=off go list -m all | grep "${repo}" | cut -d" " -f2
  )
}

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
    curl -OL --fail "https://github.com/pulumi/pulumi-${lang}/releases/download/${tag}/${filename}"
  fi
}

# shellcheck disable=SC2043
for i in "github.com/pulumi/pulumi-java java" "github.com/pulumi/pulumi-yaml yaml v1.12.0" "github.com/pulumi/pulumi-dotnet dotnet v3.71.0"; do
  set -- $i # treat strings in loop as args
  REPO="$1"
  PULUMI_LANG="$2"
  TAG="$3" # only dotnet sets this because we don't currently have a go dependency on dotnet (and quite possibly never will)
  if [ -z "$TAG" ]; then
    TAG=$(get_version "${REPO}")
  fi

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
