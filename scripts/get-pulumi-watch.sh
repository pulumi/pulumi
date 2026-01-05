#!/usr/bin/env bash

set -eo pipefail
set -x

# When run by goreleaser, the arguments are absent, so files are installed in:
#
# * ./bin/darwin-x64
# * ./bin/darwin-arm64
# * ./bin/linux-x64
# * ./bin/linux-arm64
# * ./bin/windows-x64
#
# Allowing us to customize the archives for each.
LOCAL="${1:-"false"}"

TAG="v0.1.4"

for i in \
  "darwin-x64     x86_64-apple-darwin            tar.gz" \
  "darwin-arm64   aarch64-apple-darwin           tar.gz" \
  "linux-x64      x86_64-unknown-linux-gnu       tar.gz" \
  "linux-arm64    aarch64-unknown-linux-gnu      tar.gz" \
  "windows-x64    x86_64-pc-windows-msvc         zip"; do # Windows is synonymous with ".exe" as well
  set -- $i # read loop strings as args
  TARGET="$1"
  FILE="$2"
  EXT="$3"

  GO_TARGET="${TARGET/x64/amd64}"

  DIST_DIR="./bin/${TARGET}"
  if [ "${LOCAL}" = "local" ] && [ "$(go env GOOS)-$(go env GOARCH)" != "${GO_TARGET}" ]; then
    continue
  fi

  mkdir -p "${DIST_DIR}"

  FILENAME="pulumi-watch-${TAG}-${FILE}.${EXT}"

  OUTDIR="$(mktemp -d)"
  case "${EXT}" in
    "tar.gz")
      curl -OL --fail --retry 3 "https://github.com/pulumi/watchutil-rs/releases/download/${TAG}/${FILENAME}"
      tar -xzvf "${FILENAME}" --strip-components=1 -C "${OUTDIR}"
      mv "${OUTDIR}/pulumi-watch" "${DIST_DIR}"
      rm "${FILENAME}"
      ;;
    "zip")
      curl -OL --fail --retry 3 "https://github.com/pulumi/watchutil-rs/releases/download/${TAG}/${FILENAME}"
      unzip -j "${FILENAME}" -d "${OUTDIR}"
      mv "${OUTDIR}/pulumi-watch.exe" "${DIST_DIR}"
      rm "${FILENAME}"
      ;;
    *) exit
      ;;
  esac
done
