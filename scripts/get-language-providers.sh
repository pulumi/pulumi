#!/bin/bash

set -eo pipefail
set -x

get_version() {
  repo="$1"
  (
    cd pkg
    GOWORK=off go list -m all | grep "${repo}" | cut -d" " -f2
  )
}

# This curl wrapper follows redirects, re-uses the download
# name as the filename, silences errors, and retries on
# transient network errors.
try_curl() {
  local -r TARGET_URL="$1"
  curl \
    --remote-name \
    --location \
    --fail \
    --retry 3 \
    --retry-delay 10 \
    "${TARGET_URL}"
}

# shellcheck disable=SC2043
for i in "github.com/pulumi/pulumi-java java" "github.com/pulumi/pulumi-yaml yaml"; do
  set -- $i # treat strings in loop as args
  REPO="$1"
  PULUMI_LANG="$2"
  TAG=$(get_version "${REPO}")

  LANG_DIST="goreleaser-lang/${PULUMI_LANG}"
  mkdir -p "$LANG_DIST"
  (
    # Run in a subshell to ensure we don't alter current working directory.
    cd "${LANG_DIST}"

    rm -rf ./*

    # Currently avoiding a dependency on GH CLI in favor of curl, so
    # that this script works in the context of the Brew formula:
    #
    # https://github.com/Homebrew/homebrew-core/blob/master/Formula/pulumi.rb
    #
    # Formerly:
    #
    # gh release download "${TAG}" --repo "pulumi/pulumi-${PULUMI_LANG}"

    for DIST_OS in darwin linux windows; do
      for i in "amd64 x64" "arm64 arm64"; do
        set -- $i # treat strings in loop as args
        DIST_ARCH="$1"
        RENAMED_ARCH="$2" # goreleaser in pulumi/pulumi renames amd64 to x64

        ARCHIVE="pulumi-language-${PULUMI_LANG}-${TAG}-${DIST_OS}-${DIST_ARCH}"

        # No consistency on whether Windows archives use .zip or
        # .tar.gz, try both.
        try_curl("https://github.com/pulumi/pulumi-${PULUMI_LANG}/releases/download/${TAG}/${ARCHIVE}.tar.gz") \
        || try_curl("https://github.com/pulumi/pulumi-${PULUMI_LANG}/releases/download/${TAG}/${ARCHIVE}.zip")

        OUTDIR="$DIST_OS-$RENAMED_ARCH"

        mkdir -p $OUTDIR
        find '.' -name "*-$DIST_OS-$DIST_ARCH.tar.gz" -print0 -exec tar -xzvf {} -C $OUTDIR \;
        find '.' -name "*-$DIST_OS-$DIST_ARCH.zip" -print0 -exec unzip {} -d $OUTDIR \;
      done
    done
  )
done
