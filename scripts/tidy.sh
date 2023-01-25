#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR=$(git rev-parse --show-toplevel)
SDK_DIR="$ROOT_DIR/sdk"
PKG_DIR="$ROOT_DIR/pkg"

relpath() {
  realpath=realpath
  if command -v grealpath >/dev/null; then
    realpath=grealpath
  fi

  if ! "$realpath" --relative-to="$1" "$2"; then
    echo >&2 "Could not calculate relative path."
    echo >&2 "If you're on macOS, make sure to run:"
    echo >&2 "  brew install coreutils"
    return 1
  fi
}

go_mod_update() (
  # subshell to restore dir on errors
  set -euo pipefail

  # ignore Go workspaces
  export GOWORK=off

  DIR="$1"
  pushd "$DIR" >/dev/null
  case "${DIR}" in
  # tidy only, it's not the purpose of this script to update these modules
  sdk | pkg | tests | developer-docs)
    echo "tidying '${DIR}'"
    go mod tidy -compat=1.18
    ;;
  # all other modules are test modules, and we ensure their transitive deps match pkg & sdk by
  # adding replaces, tidying, and removing.
  #
  # removing the replace directives ensures that these go modules can be copied to a temp directory
  # and run via the ProgramTest integration testing harness, as relative paths won't resolve
  # correctly following the copy.
  *)
    echo "updating and tidying '${DIR}'"
    go mod edit -replace "github.com/pulumi/pulumi/sdk/v3=$(relpath . "${SDK_DIR}")"
    go mod edit -replace "github.com/pulumi/pulumi/pkg/v3=$(relpath . "${PKG_DIR}")"
    go get -u github.com/pulumi/pulumi/sdk/v3 >/dev/null 2>&1
    go get -u github.com/pulumi/pulumi/pkg/v3 >/dev/null 2>&1
    go mod tidy -compat=1.18
    go mod edit -dropreplace "github.com/pulumi/pulumi/sdk/v3"
    go mod edit -dropreplace "github.com/pulumi/pulumi/pkg/v3"
    go mod tidy -compat=1.18
    ;;
  esac
  popd >/dev/null
)

# Update SDK first, then packages that depend on it
go_mod_update sdk
go_mod_update pkg
go_mod_update tests
go_mod_update developer-docs

# Update integration and automation tests, which must be at least one directory deeper.
for f in $(git ls-files '*/**/go.mod'); do
  go_mod_update "$(dirname "${f}")" &
done
wait
