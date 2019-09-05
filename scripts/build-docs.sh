#!/bin/bash
# build_docs.sh updates the docs generated from this repo
set -o nounset
set -o errexit
set -o pipefail
readonly ROOT=$(dirname "${0}")/..

if [[ "${TRAVIS_PUBLISH_PACKAGES:-}" == "true" ]] && [[ ! -z "${TRAVIS_TAG:-}" ]]; then
    NPM_VERSION=$("${ROOT}/scripts/get-version")
    VERSION=${NPM_VERSION#"v"}

    echo "Building SDK docs for version ${VERSION}:"

    # Clone the repo and fetch its dependencies.
    git clone "https://github.com/pulumi/docs.git" "$(go env GOPATH)/src/github.com/pulumi/docs"
    cd "$(go env GOPATH)/src/github.com/pulumi/docs"
    make ensure

    go get -u github.com/cbroglie/mustache
    go get -u github.com/gobuffalo/packr
    go get -u github.com/pkg/errors

    # Regenerate the Node.JS SDK docs
    PKGS=pulumi NOBUILD=true ./scripts/run_typedoc.sh

    # Regenerate the Python docs
    ./scripts/generate_python_docs.sh

    # Regenerate the CLI docs
    pulumi gen-markdown ./content/docs/reference/cli

    # Update latest-version
    echo -n "${VERSION}" > static/latest-version

    # Update the version list
    NL=$'\n' 
    sed -e "s/<tbody>/<tbody>\\${NL}        {{< changelog-table-row version=\"${VERSION}\" date=\"$(date +%Y-%m-%d)\" >}}/" -i ./content/docs/get-started/install/versions.md

    # Commit the resulting changes
    git checkout -b "pulumi/${TRAVIS_JOB_NUMBER}"
    git config user.name "Pulumi Bot"
    git config user.email "bot@pulumi.com"
    git add .
    git commit --allow-empty -m "Regen docs for pulumi/pulumi@${VERSION}"

    # If we have a token for pulumi-bot, push up the changes and add a status
    # to a github compare.
    if [ ! -z "${PULUMI_BOT_GITHUB_API_TOKEN:-}" ]; then
        # Push up the resulting changes
        git remote add pulumi-bot "https://pulumi-bot:${PULUMI_BOT_GITHUB_API_TOKEN}@github.com/pulumi-bot/docs"
        git push pulumi-bot --set-upstream --force "pulumi/${TRAVIS_JOB_NUMBER}"

        # Create a pull request in the docs repo.
        BODY="{\"title\": \"Regen docs for pulumi/pulumi@${VERSION}\", \"head\": \"pulumi-bot:pulumi/${TRAVIS_JOB_NUMBER}\", \"base\": \"master\"}"
        curl -u "pulumi-bot:${PULUMI_BOT_GITHUB_API_TOKEN}" -X POST -H "Content-Type: application/json" -d "${BODY}" "https://api.github.com/repos/pulumi/docs/pulls"
    else
        # Otherwise, just print out the diff to the build log.
        git diff HEAD~1 HEAD
    fi
fi

exit 0
