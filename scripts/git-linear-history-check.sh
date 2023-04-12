#!/usr/bin/env bash

# This script is designed to run against a merge commit, either produced during PR checks or a
# `bors` octopus merge commit. A non-zero exit status is returned if either of the following is true
# of the COMMITISH (see below)
# * If COMMITISH is not a merge commit (i.e.: has 0 or 1 parents.)
# * If COMMITISH is a merge commit and any of the parents have a non-linear history with the common ancestor
#   of the target commit (first parent).

# usage: ./scripts/git-linear-history-check.sh [COMMITISH]
#
#   COMMITISH: a commit to use, if not provided, HEAD is used.
#

# ## Non-linear merge example
#
# Merge commit hash: fc7c341c38c006f96ac288ebfcf5ce18b8e31a48
#
# PR commits: https://github.com/pulumi/pulumi/pull/11095/commits
#
# This commit is a single merge commit produced by `bors`. The 2nd parent of the commit is the PR
# commit HEAD and the commit history shows a merge commit. This script should log errors and exit
# with a 1 on this commit.

# ## Linear merge example
#
# Merge commit hash: 2a98a6e4dc36524fde5d33f2b5bdca0521441c72
#
# PR commits: https://github.com/pulumi/pulumi/pull/11261/commits
#
# This is a regular PR merge, containing a single commit. This script should not log any errors on
# this commit.


# ## Non-linear octopus merge example
#
# Merge commit hash: 0f3e53688fe04ec18180ba87f6915c454023ddf9
#
# PR commits:
# 1. https://github.com/pulumi/pulumi/pull/10687/commits
# 2. https://github.com/pulumi/pulumi/pull/10729/commits
# 3. https://github.com/pulumi/pulumi/pull/10740/commits
#
# This octopus merge has 3 PRs as parents. The first of which (#10687) has non-linear commit
# history. This script should log two errors and exit with a 1.


# ## Linear octopus merge example
#
# Merge commit hash: f033d9de02a633ad386b09d6dfff810ffe7ddea5
#
# PR commits:
# 1. https://github.com/pulumi/pulumi/pull/10815/commits
# 2. https://github.com/pulumi/pulumi/pull/10821/commits
# 3. https://github.com/pulumi/pulumi/pull/10822/commits
# 4. https://github.com/pulumi/pulumi/pull/10823/commits
#
# This octopus merge has 4 PRs as parents, all of which have linear commit history. This script
# should not log any errors on this commit.


# ## Initial commit example
#
# Commit hash: 86f6117640ebaaffb8689e241a668218b24f4690
#
# This commit has no parents, and should exit with a 1.


# ## Single parent (non-merge commit) example
#
# Commit hash: 1f861c5132a738216c69398ae600e1998c4e436b
#
# This commit has only a single parent, and should exit with a 1.

COMMITISH="${1:-"HEAD"}"
MERGED_BRANCH_COMMIT="$(git rev-parse "${COMMITISH}")"

>&2 echo "Checking merge commit ${MERGED_BRANCH_COMMIT} for non-linear history"

# git rev-list - list the merged branch commit followed by all of its parents, separated by spaces
# cut          - remove the first line
# tr           - replace spaces with newlines to turn this into an array
PARENTS_RAW=$(git rev-list --no-commit-header --parents -n 1 "${MERGED_BRANCH_COMMIT}" | cut -d' ' -f2- | tr ' ' '\n')

readarray -t PARENTS <<<"${PARENTS_RAW}" # split into array on newlines, -t strips newlines
if [ "${#PARENTS[@]}" -le "1" ]; then
  >&2 echo "::error::Input commit ${COMMITISH} is not a merge commit, this script must run against a merge commit."
  exit 1
fi

# First parent in bors & github PR merge commits is always the target branch's HEAD commit:
TARGET_BRANCH_HEAD="${PARENTS[0]}"
>&2 echo "Main branch parent is: $(git log --oneline -n 1 "${TARGET_BRANCH_HEAD}")"
# Subsequent parents are from PR branches:
PR_BRANCH_HEADS=( "${PARENTS[@]:1}" )
>&2 echo "PR branch parents are ${PR_BRANCH_HEADS[*]}"

HAS_MERGE_COMMIT=false
for PR_COMMIT in "${PR_BRANCH_HEADS[@]}"; do
  >&2 echo "Checking: $(git log --oneline -n 1 "${PR_COMMIT}")"
  # Find the common parent of the target branch and PR branch
  MERGE_COMMITS_IN_PR=$(git rev-list "${TARGET_BRANCH_HEAD}..${PR_COMMIT}" --merges | cut -d' ' -f2-)
  for MERGE_COMMIT in ${MERGE_COMMITS_IN_PR}; do
    >&2 echo "::error::Non-linear history, PR contains a merge ${MERGE_COMMIT}. Remove this by rebasing on the target."
    HAS_MERGE_COMMIT=true
  done
done

if ${HAS_MERGE_COMMIT}; then
  >&2 echo "::error::Detected non-linear history."
  exit 1
fi

>&2 echo "✅ Commit history is linear."
