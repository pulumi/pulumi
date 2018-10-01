#!/bin/bash
# This is an entrypoint for our Docker image that does some minimal bootstrapping before executing.

set -e

# If the PULUMI_CI variable is set, we'll do some extra things to make common tasks easier.
if [ ! -z "$PULUMI_CI" ]; then
    # Detect the CI system and configure variables so that we get good Pulumi workflow and GitHub App support.
    if [ ! -z "$GITHUB_WORKFLOW" ]; then
        REF="$GITHUB_REF"
        export PULUMI_CI_SYSTEM="GitHub"
        export PULUMI_CI_BUILD_ID=
        export PULUMI_CI_BUILD_TYPE=
        export PULUMI_CI_BUILD_URL=
        export PULUMI_CI_PULL_REQUEST_SHA="$GITHUB_SHA"
    fi

    # Respect the branch mappings file for stack selection. Note that this is *not* required, but if the file
    # is missing, the caller of this script will need to pass `-s <stack-name>` to specify the stack explicitly.
    if [ ! -z "$REF" ] && [ -e .pulumi/ci.json ]; then
        PULUMI_STACK_NAME=$(cat .pulumi/ci.json | jq -r ".\"$REF\"")
        if [ "$PULUMI_STACK_NAME" != "null" ]; then
            pulumi stack select $PULUMI_STACK_NAME
        else
            echo "No stack configured for branch '$REF'"
            echo ""
            echo "To configure this branch, please"
            echo "\t1) Run 'pulumi stack init <stack-name>'"
            echo "\t2) Associated the stack with the branch by adding"
            echo "\t\t{"
            echo "\t\t\t\"$REF\": \"<stack-name>\""
            echo "\t\t}"
            echo "\tto your .pulumi/ci.json file"
            echo ""
            echo "For now, exiting cleanly without doing anything..."
            exit 0
        fi
    fi
fi

# Next, lazily install packages if required.
if [ -e package.json ] && [ ! -d node_modules ]; then
    npm install
fi

# Now just pass along all arguments to the Pulumi CLI.
pulumi "$@"
