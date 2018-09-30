#!/bin/bash
# This is an entrypoint for our Docker image that does some minimal bootstrapping before executing.

# First, lazily install packages if required.
if [ -e package.json ] && [ ! -d node_modules ]; then
    npm install
fi

# Now just pass along all arguments to the Pulumi CLI.
pulumi "$@"
