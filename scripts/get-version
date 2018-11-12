#!/bin/bash
set -o nounset -o errexit -o pipefail
SCRIPT_DIR="$( cd "$( dirname "$0" )" && pwd )"
COMMITISH=${1:-HEAD}
DIRTY_TAG=""

# Figure out if the worktree is dirty, we run update-index first
# as we've seen cases in Travis where not doing so causes git to
# treat the worktree as dirty when it is not.
git update-index -q --refresh
if ! git diff-files --quiet; then
    DIRTY_TAG="dirty"
fi

# If we have an exact tag, just use it.
if git describe --tags --exact-match "${COMMITISH}" >/dev/null 2>&1; then
    echo -n "$(git describe --tags --exact-match "${COMMITISH}")"
    if [ ! -z "${DIRTY_TAG}" ]; then
        echo -n "+${DIRTY_TAG}"
    fi

    echo ""
    exit 0
fi

# Otherwise, increment the patch version, add the -dev tag and some
# commit metadata. If there's no existing tag, pretend a v0.0.0 was
# there so we'll produce v0.0.1-dev builds.
if git describe --tags --abbrev=0 "${COMMITISH}" > /dev/null 2>&1; then
    TAG=$(git describe --tags --abbrev=0 "${COMMITISH}")
else
    TAG="v0.0.0"
fi

# Strip off any pre-release tag we might have (e.g. from doing a -rc build)
TAG=${TAG%%-*}

MAJOR=$(cut -d. -f1 <<< "${TAG}")
MINOR=$(cut -d. -f2 <<< "${TAG}")
PATCH=$(cut -d. -f3 <<< "${TAG}")

# We want to include some additional information. To the base tag we
# add a timestamp and commit hash. We use the timestamp of the commit
# itself, not the date it was authored (so it will change when someone
# rebases a PR into master, for example).
echo -n "${MAJOR}.${MINOR}.$((${PATCH}+1))-dev.$(git show -s --format='%ct+g%h' ${COMMITISH})"
if [ ! -z "${DIRTY_TAG}" ]; then
    echo -n ".${DIRTY_TAG}"
fi

echo ""
