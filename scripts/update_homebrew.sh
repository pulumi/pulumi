#!/bin/bash
# update_homebrew.sh uses `brew bump-formula-pr` to update the formula for the Pulumi CLI and SDKs
set -o nounset
set -o errexit
set -o pipefail
readonly ROOT=$(dirname "${0}")/..

if [[ "${TRAVIS:-}" != "true" ]]; then
    echo "error: this script should be run from within Travis"
    exit 1
fi

if [[ -z "${PULUMI_BOT_GITHUB_API_TOKEN:-}" ]]; then
    echo "error: PULUMI_BOT_GITHUB_API_TOKEN must be set"
    exit 1
fi

if ! echo "${TRAVIS_TAG:-}" | grep -q -e "^v[0-9]\+\.[0-9]\+\.[0-9]\+$"; then
    echo "Skipping Homebrew formula update; ${TRAVIS_TAG:-} does not denote a released version"
    exit 0
fi

if [[ "${TRAVIS_OS_NAME:-}" != "osx" ]]; then
    echo "Skipping Homebrew formula updte; not running on OS X"
    exit 0
fi

HOMEBREW_GITHUB_API_TOKEN="${PULUMI_BOT_GITHUB_API_TOKEN:-}" brew bump-formula-pr --tag="${TRAVIS_TAG:-}" --revision="${TRAVIS_COMMIT:-}" pulumi
exit 0
