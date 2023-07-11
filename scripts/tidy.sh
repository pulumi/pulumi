#!/usr/bin/env bash

set -euo pipefail

# Array of paths to exclude from go mod tidy.
# Files that are intentionally malformed should be added here
# with an explanation of why they are excluded.
EXCLUDE=(
    # This issue occurs only if a package referenced in the go.mod
    # is not actually imported by the accompanying Go code.
    #
    # 'go mod tidy' will remove the unused dependency,
    # but we want to keep it in the go.mod file.
    tests/integration/go/regress-13301 \
)

# Reads from stdin and filters out any paths that are in the EXCLUDE array.
filter_excludes() {
    # Explanation:
    #  -v: Invert the match, i.e. print lines that don't match
    #  -F: Treat each line as an exact string, not a regex
    #  -f: Read patterns from the file
    #  <(..): Treat the output of the command inside the (..) as a file
    #  printf: Print each element of the array on a separate line
    #
    # Ref: https://askubuntu.com/a/1446204
    grep -vFf <(printf '%s\n' "${EXCLUDE[@]}")
}

for f in $(git ls-files | grep go.mod | filter_excludes)
do
    (cd "$(dirname "${f}")" && go mod tidy -compat=1.18)
done
