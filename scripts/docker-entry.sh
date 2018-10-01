#!/bin/bash
# This is an entrypoint for our Docker image that does some minimal bootstrapping before executing.

# Detect the CI system and configure variables so that we get good Pulumi workflow and GitHub App support.
if [ ! -z "${GITHUB_WORKFLOW}" ]; then
    export PULUMI_CI_SYSTEM="GitHub"
    export PULUMI_CI_BUILD_ID=
    export PULUMI_CI_BUILD_TYPE=
    export PULUMI_CI_BUILD_URL=
    export PULUMI_CI_PULL_REQUEST_SHA="${GITHUB_SHA}"
fi

# Next, lazily install packages if required.
if [ -e package.json ] && [ ! -d node_modules ]; then
    npm install
fi

# Now just pass along all arguments to the Pulumi CLI.
pulumi "$@"
