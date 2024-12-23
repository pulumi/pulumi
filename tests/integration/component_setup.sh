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

setup_python() (
  set -euo pipefail
  if [ -d "testcomponent-python" ]; then
    cd testcomponent-python
    # Clear out any existing venv to prevent 'permission denied' issues
    python3 -m venv venv --clear
    # shellcheck disable=SC1090
    . venv/*/activate
    # In Python 3.13.0, the virtualenv activate script does not correctly detect
    # that we're on Windows when running in Git Bash. Use a path to the python
    # executable in the virtualenv to work around this issue.
    # https://github.com/python/cpython/issues/125398
    pythonBin=$(command -v python)
    if [[ ${OSTYPE} == "msys" ]]; then
        pythonBin=$(cygpath -u "./venv/Scripts/python.exe")
    fi
    ${pythonBin} -m pip install -e ../../../../sdk/python
  fi
)

setup_nodejs
setup_python

i=0
for step in setup_nodejs setup_python; do
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
