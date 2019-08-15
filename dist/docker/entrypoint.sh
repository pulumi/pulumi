#!/bin/bash

set -e

# Install Node dependencies if a package.json is present.
if [ -e package.json ]; then
    npm install
fi

# Install Python dependencies if a Pipfile is present.
if [ -e Pipfile ]; then
    pip install -r requirements.txt
fi

# Pass any additional arguments to the pulumi executable.
pulumi $*
