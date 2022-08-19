#!/bin/bash

set -eo pipefail
set -x

# When run by goreleaser, the arguments are absent, so files are installed in:
#
# * ./bin/darwin-amd64
# * ./bin/darwin-arm64
# * ./bin/linux-amd64
# * ./bin/linux-arm64
# * ./bin/windows-amd64
#
# Allowing us to customize the archives for each.
#
# When run by GitHub Actions in tests, we set the args so that we install only the current OS's
# binaries and in the shared "local path" dir, ./bin
FILTER_TARGET="${1}"

if [ "${FILTER_TARGET}" = "local" ]; then
  FILTER_TARGET="$(go env GOOS)-$(go env GOARCH)"
fi

TAG="v0.1.4"

for i in \
  "darwin-amd64   x86_64-apple-darwin            tar.gz" \
  "darwin-arm64   aarch64-apple-darwin           tar.gz" \
  "linux-amd64    x86_64-unknown-linux-gnu       tar.gz" \
  "linux-arm64    aarch64-unknown-linux-gnu      tar.gz" \
  "windows-amd64  x86_64-pc-windows-msvc         zip"; do # Windows is synonymous with ".exe" as well
  set -- $i # read loop strings as args
  TARGET="$1"
  FILE="$2"
  EXT="$3"

  DIST_DIR="./bin/${TARGET}"
  if [ -n "${FILTER_TARGET}" ]; then
    if [ "${TARGET}" != "${FILTER_TARGET}" ]; then
      continue
    else
      DIST_DIR="./bin"
    fi
  fi

  mkdir -p "${DIST_DIR}"

  FILENAME="pulumi-watch-${TAG}-${FILE}.${EXT}"

  OUTDIR="$(mktemp -d)"
  case "${EXT}" in
    "tar.gz")
      curl -OL --fail "https://github.com/pulumi/watchutil-rs/releases/download/${TAG}/${FILENAME}"
      tar -xzvf "${FILENAME}" --strip-components=1 -C "${OUTDIR}"
      mv "${OUTDIR}/pulumi-watch" "${DIST_DIR}"
      rm "${FILENAME}"
      ;;
    "zip")
      curl -OL --fail "https://github.com/pulumi/watchutil-rs/releases/download/${TAG}/${FILENAME}"
      unzip -j "${FILENAME}" -d "${OUTDIR}"
      mv "${OUTDIR}/pulumi-watch.exe" "${DIST_DIR}"
      rm "${FILENAME}"
      ;;
    *) exit
      ;;
  esac
done
