#!/usr/bin/env bash

set -euo pipefail

usage()
{
    echo "Usage: ./scripts/release.sh start VERSION"
    echo "   or: ./scripts/release.sh push  VERSION"
    echo "   or: ./scripts/release.sh tag   VERSION"
    echo ""
    echo "Automates release-related chores"
    echo ""
    echo "Example releasing v3.35.1:"
    echo ""
    echo "./scripts/release.sh start v3.35.1"
    echo "# follow instructions to edit CHANGELOG.md"
    echo "./scripts/release.sh push v3.35.1"
    echo "# follow instructions to merge release/v3.35.1 branch to master"
    echo "./scripts/release.sh tag v3.35.1"
}

if [ "$#" -ne 2 ]; then
    usage
    exit 1
fi

VERSION="$2"

case "$1" in
    start)
        git fetch origin master
        git checkout master -b release/${VERSION}
        echo "Please edit CHANGELOG.md to add a ${VERSION} section with CHANGELOG_PENDING.md changes."
        echo "When done, run the following to commit and push the changes:"
        echo ""
        echo "    ./scripts/release.sh push"
        ;;
    push)
        git add CHANGELOG.md
        git commit -m "Prepare for ${VERSION} release"

        VERSION="$2"
        (cd pkg   && go mod edit -require github.com/pulumi/pulumi/sdk/v3@${VERSION})
        (cd tests && go mod edit -require github.com/pulumi/pulumi/sdk/v3@${VERSION})
        make tidy

        git add pkg
        git add tests
        git commit -m "Release ${VERSION}"

        echo "### Improvements" >  CHANGELOG_PENDING.md
        echo ""                 >> CHANGELOG_PENDING.md
        echo "### Bug Fixes"    >> CHANGELOG_PENDING.md
        echo ""                 >> CHANGELOG_PENDING.md

        git add CHANGELOG_PENDING.md
        git commit -m "Cleanup after ${VERSION} release"

        git push --set-upstream origin release/${VERSION}

        echo "Open a PR and merge release/${VERSION} branch to master."
        echo "Make sure to use 'Rebase and merge' option to preserve the commit sequence."
        echo "When done, run the following to finish the release:"
        echo ""
        echo "    ./scripts/release.sh tag ${VERSION}"

        ;;
    tag)
        git fetch origin master

        SDK_COMMENT=$(git log --format=%B -n 1 origin/master~2)
        REL_COMMENT=$(git log --format=%B -n 1 origin/master~1)

        if [ "$REL_COMMENT" != "Release ${VERSION}" ]; then
            echo "Aborting, expected origin/master~2 comment to be 'Release ${VERSION}' but got '${REL_COMMENT}'"
            exit 1
        fi

        if [ "$SDK_COMMENT" != "Prepare for ${VERSION} release" ]; then
            echo "Aborting, expected origin/master~1 comment to be 'Prepare for ${VERSION} release' but got '${SDK_COMMENT}'"
            exit 1
        fi

        git tag sdk/${VERSION} origin/master~2
        git tag pkg/${VERSION} origin/master~1
        git tag ${VERSION}     origin/master~1
        git push origin sdk/${VERSION}
        git push origin pkg/${VERSION}
        git push origin ${VERSION}
        ;;
    *)
        echo "Invalid command: $1. Expecting one of: start, push, tag"
        usage
        exit 1
        ;;
esac
