#!/bin/bash

set -eo pipefail
set -x

# shellcheck disable=SC2043
for i in "java v0.1.0" "yaml v0.3.0"; do
  set -- $i # treat strings in loop as args
  PULUMI_LANG="$1"
  TAG="$2"

  LANG_DIST="goreleaser-lang/${PULUMI_LANG}"
  mkdir -p "$LANG_DIST"
  cd "${LANG_DIST}"

  rm -rf ./*
  gh release download "${TAG}" --repo "pulumi/pulumi-${PULUMI_LANG}"

  for DIST_OS in darwin linux windows; do
    for DIST_ARCH in amd64 arm64; do
      DIR="$DIST_OS-$DIST_ARCH"
      mkdir -p $DIR
      find '.' -name "*-$DIST_OS-$DIST_ARCH.tar.gz" -print0 -exec tar -xzvf {} -C $DIR \;
      find '.' -name "*-$DIST_OS-$DIST_ARCH.zip" -print0 -exec unzip {} -d $DIR \;
    done
  done

  cd ../..
done
