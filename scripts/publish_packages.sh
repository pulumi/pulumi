#!/bin/bash
# publish_packages.sh uploads our packages to package repositories like npm
set -o nounset -o errexit -o pipefail
ROOT=$(dirname $0)/..

if [[ "${TRAVIS_OS_NAME:-}" == "linux" ]]; then
    echo "Publishing NPM package to NPMjs.com:"
    pushd ${ROOT}/sdk/nodejs/bin && \
        npm publish && \
        npm info 2>/dev/null || true && \
        popd

    echo "Publishing Pip package to pulumi.com:"
    twine upload \
        --repository-url https://pypi.pulumi.com?token=${PULUMI_API_TOKEN} \
        -u pulumi -p pulumi \
        ${ROOT}/sdk/python/bin/dist/*.whl
fi
