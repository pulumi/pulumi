#!/bin/bash

set -euo pipefail

SCRIPTDIR=$(dirname "$0")
REPODIR=$(dirname "${SCRIPTDIR}")

LANGUAGE="${1:-""}"

SNAPSHOT_VERSION=""
if [ -z "${PULUMI_VERSION:-""}" ]; then
  SNAPSHOT_VERSION="-alpha.$(date +%s)"
fi

VERSION="${PULUMI_VERSION:-$(cat "${REPODIR}/.version")}"

case "${LANGUAGE}" in
  python)
    echo -n "${VERSION}${SNAPSHOT_VERSION}" | sed 's/-alpha./a/; s/-beta./b/; s/-rc./rc/'
    ;;
  *)
    echo -n "${VERSION}${SNAPSHOT_VERSION}"
    ;;
esac
