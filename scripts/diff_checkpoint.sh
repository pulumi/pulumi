#!/bin/bash
find_snapshots() {
    find . -name "*.json.*" | sort
}

revisionNumber=$1
preMutation=$revisionNumber
postMutation=$(($revisionNumber + 1))
preMutationFile=$(find_snapshots | sed -n "${preMutation}p")
postMutationFile=$(find_snapshots | sed -n "${postMutation}p")
if hash colordiff 2>/dev/null; then
    colordiff $preMutationFile $postMutationFile
else
    diff $preMutationFile $postMutationFile
fi

