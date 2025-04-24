#!/usr/bin/env bash

set -euo pipefail

SCRIPTDIR=$(dirname "$0")
REPODIR=$(dirname "${SCRIPTDIR}")

LANGUAGE="${1:-""}"

if [ -z "${PULUMI_VERSION:-""}" ]; then
  VERSION="${PULUMI_VERSION:-$("${REPODIR}/.github/scripts/get-version")}"
  VERSION="${VERSION%-*}" # remove tags
  VERSION="${VERSION}-dev.0"
else
  VERSION="${PULUMI_VERSION:-$("${REPODIR}/.github/scripts/get-version")}"
fi

case "${LANGUAGE}" in
  python)
    echo -n "${VERSION}" | sed "s/-alpha.*/a$(date +%s)/"
    ;;
  *)
    echo -n "${VERSION}"
    ;;
esac
