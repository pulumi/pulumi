#!/bin/bash

set -eo pipefail
set -x

# shellcheck disable=SC2043
for i in "java v0.4.1" "yaml v0.5.2"; do
  set -- $i # treat strings in loop as args
  PULUMI_LANG="$1"
  TAG="$2"

  LANG_DIST="goreleaser-lang/${PULUMI_LANG}"
  mkdir -p "$LANG_DIST"
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

      curl -OL --fail "https://github.com/pulumi/pulumi-${PULUMI_LANG}/releases/download/${TAG}/${ARCHIVE}.tar.gz" || echo "ignoring download"
      curl -OL --fail "https://github.com/pulumi/pulumi-${PULUMI_LANG}/releases/download/${TAG}/${ARCHIVE}.zip" || echo "ignoring download"

      OUTDIR="$DIST_OS-$RENAMED_ARCH"

      mkdir -p $OUTDIR
      find '.' -name "*-$DIST_OS-$DIST_ARCH.tar.gz" -print0 -exec tar -xzvf {} -C $OUTDIR \;
      find '.' -name "*-$DIST_OS-$DIST_ARCH.zip" -print0 -exec unzip {} -d $OUTDIR \;
    done
  done

  cd ../..
done
