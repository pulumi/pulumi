#!/usr/bin/env bash
set -eu

args=("$@")

# Make covprofile handle re-runs safely.
for arg in "$@"; do
    if [[ $arg == "-coverprofile="* ]]; then
        args+=("$arg.${RANDOM}.cov")
    else
        args+=("$arg")
    fi
done

go test "${args[@]}"
