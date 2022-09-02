#!/bin/bash

set -euo pipefail

SCRIPTDIR=$(dirname "$0")
REPODIR=$(dirname "${SCRIPTDIR}")

LANGUAGE="${1:-""}"

if [ -z "${PULUMI_VERSION:-""}" ]; then
  VERSION="${PULUMI_VERSION:-$("${REPODIR}/.github/scripts/get-version")}"
  VERSION="${VERSION%-*}" # remove tags
  TIMESTAMP="-alpha.$(date +%s)"
  VERSION="${VERSION}${TIMESTAMP}"
else
  VERSION="${PULUMI_VERSION:-$("${REPODIR}/.github/scripts/get-version")}"
fi

case "${LANGUAGE}" in
  python)
    echo -n "${VERSION}" | sed 's/-alpha./a/; s/-beta./b/; s/-rc./rc/'
    ;;
  *)
    echo -n "${VERSION}"
    ;;
esac
