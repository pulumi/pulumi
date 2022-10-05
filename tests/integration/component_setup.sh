#!/usr/bin/env bash

# Setup script for components which require a build step. Runs install steps concurrently and with a
# lock. Enables us to run tests via makefile without blocking.

set -euo pipefail

DONEFILE=".done"
EXPECTED_HASH="$(git rev-parse HEAD:./)"

BUILD=false
if [ -f "${DONEFILE}" ] && [ "$(head "${DONEFILE}")" = "${EXPECTED_HASH}" ]; then
  echo "Setup marked complete"
else
  echo "Setup not marked done"
  BUILD=true
fi

if output=$(git status --porcelain .) && [ -z "${output}" ]; then
  echo "Directory is clean"
else
  echo "Directory is dirty"
  BUILD=true
fi

if ! "${BUILD}"; then
  echo "Setup already done, directory is clean. Success!"
  exit 0
fi


setup_nodejs() (
  set -euo pipefail
  if [ -d "testcomponent" ]; then
    cd testcomponent
    yarn link @pulumi/pulumi
    yarn install
    yarn run tsc
  fi
)

setup_go() (
  set -euo pipefail
  if [ -d "testcomponent-go" ]; then
    cd testcomponent-go
    go build -o "pulumi-resource-testcomponent$(go env GOEXE)"
    cd ..
  fi
  if [ -d "testcomponent2-go" ]; then
    cd testcomponent2-go
    go build -o "pulumi-resource-secondtestcomponent$(go env GOEXE)"
    cd ..
  fi
)

setup_python() (
  set -euo pipefail
  if [ -d "testcomponent-python" ]; then
    cd testcomponent-python
    python3 -m venv venv
    # shellcheck disable=SC1090
    . venv/*/activate
    python3 -m pip install -e ../../../../sdk/python/env/src
  fi
)

setup_nodejs
setup_go
setup_python

i=0
for step in setup_nodejs setup_python setup_go; do
  time "${step}" &
  builds[${i}]=$!
  echo "Started ${step} with PID ${builds[${i}]}"
  i=$((i+1))
done

for build_pid in "${builds[@]}"; do
  echo "Waiting for ${build_pid}"
  wait "${build_pid}"
done

echo "${EXPECTED_HASH}" > "${DONEFILE}"
