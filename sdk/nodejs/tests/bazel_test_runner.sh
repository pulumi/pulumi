#!/usr/bin/env bash
# Test runner for Node.js SDK tests under Bazel.
# Usage: bazel_test_runner.sh <test_category> [extra_mocha_args...]
#
# Categories:
#   unit       - Core unit tests (excludes automation and closure-integration)
#   automation - Automation API tests
#   mocks      - Tests with mocks
#   proto      - Proto tests

set -euo pipefail

CATEGORY="${1:?Usage: bazel_test_runner.sh <category>}"
shift

# Find the sdk/nodejs source directory.
# Strategy: locate our own script, then walk up to find the workspace root.
find_workspace() {
    # If BUILD_WORKSPACE_DIRECTORY is set and valid, use it
    if [ -n "${BUILD_WORKSPACE_DIRECTORY:-}" ] && [ -f "${BUILD_WORKSPACE_DIRECTORY}/sdk/nodejs/package.json" ]; then
        echo "${BUILD_WORKSPACE_DIRECTORY}/sdk/nodejs"
        return
    fi

    # Follow the script's real path to find the source tree
    local script_path
    script_path="$(readlink -f "${BASH_SOURCE[0]}")"
    local dir
    dir="$(dirname "$script_path")"

    # Walk up looking for sdk/nodejs/package.json
    while [ "$dir" != "/" ]; do
        if [ -f "$dir/sdk/nodejs/package.json" ]; then
            echo "$dir/sdk/nodejs"
            return
        fi
        if [ -f "$dir/package.json" ] && grep -q '@pulumi/pulumi' "$dir/package.json" 2>/dev/null; then
            echo "$dir"
            return
        fi
        dir="$(dirname "$dir")"
    done

    echo ""
}

SDK_DIR="$(find_workspace)"

if [ -z "$SDK_DIR" ] || [ ! -f "$SDK_DIR/package.json" ]; then
    echo "ERROR: Cannot find sdk/nodejs/package.json"
    echo "BUILD_WORKSPACE_DIRECTORY=${BUILD_WORKSPACE_DIRECTORY:-unset}"
    echo "BASH_SOURCE=${BASH_SOURCE[0]}"
    echo "Resolved: $(readlink -f "${BASH_SOURCE[0]}" 2>/dev/null || echo 'N/A')"
    exit 1
fi

cd "$SDK_DIR"

# Ensure dependencies are installed
if [ ! -d "node_modules" ] || [ ! -f "node_modules/.yarn-integrity" ]; then
    echo "Installing Node.js dependencies..."
    yarn install --frozen-lockfile 2>&1 || yarn install 2>&1
fi

# Ensure TypeScript is compiled
if [ ! -d "bin/tests" ] || [ "$(find tests -name '*.spec.ts' -newer bin/tests -print -quit 2>/dev/null)" ]; then
    echo "Compiling TypeScript..."
    yarn run tsc 2>&1
fi

MOCHA="yarn run mocha"

case "$CATEGORY" in
    unit)
        echo "Running Node.js unit tests..."
        $MOCHA --timeout 120000 \
            --exclude 'bin/tests/automation/**/*.spec.js' \
            --exclude 'bin/tests/runtime/closure-integration-tests.js' \
            'bin/tests/**/*.spec.js' "$@"
        ;;
    automation)
        echo "Running Node.js automation tests..."
        $MOCHA --timeout 300000 --parallel \
            'bin/tests/automation/**/*.spec.js' "$@"
        ;;
    mocks)
        echo "Running Node.js mock tests..."
        $MOCHA 'bin/tests_with_mocks/**/*.spec.js' "$@"
        ;;
    proto)
        echo "Running Node.js proto tests..."
        $MOCHA --timeout 120000 \
            'bin/tests/proto/**/*.spec.js' "$@"
        ;;
    *)
        echo "Unknown test category: $CATEGORY"
        echo "Valid categories: unit, automation, mocks, proto"
        exit 1
        ;;
esac
