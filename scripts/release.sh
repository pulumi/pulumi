#!/usr/bin/env bash

set -euo pipefail

VERSION="@1"

die()
{
    echo "$1"
    exit 1
}

usage()
{
    echo "Usage: ./scripts/release.sh update-sdk-ref VERSION"
    echo "Automates release-related chores"
}

if [ "$#" -eq 0 ]; then
    usage
    exit 1
fi

case "$1" in
    update-sdk-ref)
        VERSION="$2"
        (cd pkg   && go mod edit -require github.com/pulumi/pulumi/sdk/v3@${VERSION})
        (cd tests && go mod edit -require github.com/pulumi/pulumi/sdk/v3@${VERSION})
        make tidy
        ;;
    *)
        usage
        die "Unknown command: $1"
        ;;
esac
