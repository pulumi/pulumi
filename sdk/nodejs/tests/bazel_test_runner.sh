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

# Add Bazel-built binaries to PATH (e.g. pulumi-language-nodejs from runfiles)
if [ -n "${RUNFILES_DIR:-}" ]; then
    LANGHOST_BIN="$RUNFILES_DIR/_main/sdk/nodejs/cmd/pulumi-language-nodejs/pulumi-language-nodejs_/pulumi-language-nodejs"
    if [ -x "$LANGHOST_BIN" ]; then
        export PATH="$(dirname "$LANGHOST_BIN"):$PATH"
    fi
fi

# Ensure dependencies are installed
if [ ! -d "node_modules" ] || [ ! -f "node_modules/.yarn-integrity" ]; then
    echo "Installing Node.js dependencies..."
    yarn install --frozen-lockfile 2>&1 || yarn install 2>&1
fi

# Ensure TypeScript is compiled and build artifacts are in place
if [ ! -d "bin/tests" ] || [ "$(find tests -name '*.spec.ts' -newer bin/tests -print -quit 2>/dev/null)" ]; then
    echo "Compiling TypeScript..."
    yarn run tsc 2>&1
fi

# Ensure non-TypeScript build artifacts are copied to bin/
# (tsc only compiles .ts files; these .js and data files need manual copying)
if [ -d "proto" ]; then
    mkdir -p bin/proto
    cp -Rf proto/. bin/proto/ 2>/dev/null || true
fi
if [ -d "tests/provider/experimental/testdata" ]; then
    mkdir -p bin/tests/provider/experimental/
    cp -Rf tests/provider/experimental/testdata/ bin/tests/provider/experimental/testdata 2>/dev/null || true
fi
if [ -d "tests/runtime/langhost/cases" ]; then
    mkdir -p bin/tests/runtime/langhost/cases/
    find tests/runtime/langhost/cases/* -type d -exec cp -Rf {} bin/tests/runtime/langhost/cases/ \; 2>/dev/null || true
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
    integration)
        echo "Running Node.js integration tests..."
        node 'bin/tests/runtime/closure-integration-tests.js'
        node 'bin/tests/runtime/install-package-tests.js'
        ;;
    sxs)
        echo "Running Node.js TypeScript version compatibility tests..."
        cd tests/sxs_ts_test
        for version in "~3.8.3" "^3" "^4" "^6"; do
            cp -f "package${version}.json" package.json 2>/dev/null || continue
            project="tsconfig.json"
            if [ -f "tsconfig${version}.json" ]; then
                project="tsconfig${version}.json"
            fi
            yarn install --frozen-lockfile 2>&1 || yarn install 2>&1
            yarn run tsc --version
            yarn run tsc --project "$project"
            rm package.json
            echo "TypeScript ${version} passed"
        done
        ;;
    *)
        echo "Unknown test category: $CATEGORY"
        echo "Valid categories: unit, automation, mocks, proto, integration, sxs"
        exit 1
        ;;
esac
