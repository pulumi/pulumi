# Clean up the CLI version for dev builds, since they aren't valid tag names.
# #!/bin/bash
#
# Builds the Pulumi docker containers locally. Optionally running tests or
# publishing to a container registry.
#
# Usage: build-docker cli-version [--test] [--publish]
set -o nounset
set -o errexit
set -o pipefail

readonly SCRIPT_DIR="$( cd "$( dirname "${0}" )" && pwd )"
readonly ROOT=${SCRIPT_DIR}/..

if [ -z "${1:-}" ]; then
    >&2 echo "error: missing version to publish"
    exit 1
fi

# Sanitize the name of the version, e.g.
# "v1.14.0-alpha.1586190504+gf4e9f7e2" -> "v1.14.0-alpha.1586190504".
readonly CLI_VERSION="$(echo "${1}" | sed 's/\+.*//g')"

echo_header() {
    echo -e "\n\033[0;35m${1}\033[0m"
}

echo_header "Building local copy of Pulumi containers (${CLI_VERSION})"
echo "- Building pulumi/pulumi"
docker build --build-arg PULUMI_VERSION="${CLI_VERSION}" \
    -t "pulumi/pulumi:${CLI_VERSION}" \
    -t "pulumi/pulumi:latest" \
    "${SCRIPT_DIR}/../docker/pulumi"
echo_header "Executing container runtime tests"

# Run the container tests, note that we also build the binaries into /tmp for the next step.
pushd ${ROOT}/tests
GOOS=linux go test -c -o /tmp/pulumi-test-containers ./containers/...
popd

# Run tests _within_ the "pulumi" container, ensuring that the CLI is installed
# and working correctly.
docker run -e RUN_CONTAINER_TESTS=true \
    -e PULUMI_ACCESS_TOKEN=${PULUMI_ACCESS_TOKEN} \
    --volume /tmp:/src \
    --entrypoint /bin/bash \
    pulumi/pulumi:latest \
    -c "pip install pipenv && /src/pulumi-test-containers -test.parallel=1 -test.timeout=1h -test.v -test.run TestPulumiDockerImage"
