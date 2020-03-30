#!/bin/bash
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
CLI_VERSION="${1}"

echo_header() {
    echo -e "\n\033[0;35m${1}\033[0m"
}

# Test the containers.
test_containers() {
    # Run tests _within_ the "pulumi" container, ensuring that the CLI is installed
    # and working correctly.
    echo_header "Executing container runtime tests"
    docker run -e RUN_CONTAINER_TESTS=true \
        -e PULUMI_ACCESS_TOKEN=${PULUMI_ACCESS_TOKEN} \
        --volume /tmp:/src \
        --entrypoint /bin/bash \
        pulumi/pulumi:latest \
        -c "pip install pipenv && /src/pulumi-test-containers -test.parallel=1 -test.v -test.run TestPulumiDockerImage"

    # The actions container should fetch program dependencies from NPM, Pip, etc. before
    # executing. These tests just shell out to docker run to confirm that.
    echo_header "Executing container entrypoint tests"
    RUN_CONTAINER_TESTS=true go test ${ROOT}/tests/containers/... -test.run TestPulumiActionsImage -test.v

    # In case there are any other unit tests defined in the module, run those as well.
    GOOS=linux go test -c -o /tmp/pulumi-test-containers ${ROOT}/tests/containers/...
}

# Publishes the built containers to Docker Hub.
publish_containers() {
    echo_header "Publishing containers"

    # Required environment variables.
    if [ -z "${DOCKER_HUB_USER:-}" ]; then
        >&2 echo "error: 'DOCKER_HUB_USER' should be defined"
        exit 1
    fi

    if [ -z "${DOCKER_HUB_PASSWORD:-}" ]; then
        >&2 echo "error: 'DOCKER_HUB_PASSWORD' should be defined"
        exit 1
    fi

    # We only want to push docker images for stable versions of Pulumi. So if there is a -alpha
    # pre-release tag, skip publishing.
    if [[ "${CLI_VERSION}" == *-alpha* ]]; then
        >&2 echo "Skipping docker publishing for ${CLI_VERSION} since it is a pre-release"
        exit 0    
    fi

    docker login -u "${DOCKER_HUB_USER}" -p "${DOCKER_HUB_PASSWORD}"

    for container in ${PULUMI_CONTAINERS[@]}; do
        echo "- pulumi/${container}"
        docker push "pulumi/${container}:${CLI_VERSION}"
        docker push "pulumi/${container}:latest"
    done

    docker logout
}

# The Docker containers built/tested/published from this repository.
PULUMI_CONTAINERS=("pulumi" "actions")

echo_header "Building Pulumi containers"
for container in ${PULUMI_CONTAINERS[@]}; do
    echo "- Building pulumi/${container}"
    docker build --build-arg PULUMI_VERSION="${CLI_VERSION}" \
        -t "pulumi/${container}:${CLI_VERSION}" \
        -t "pulumi/${container}:latest" \
        "${SCRIPT_DIR}/../dist/${container}"
done

# Loop through the remaining args, running them in order.
for script_arg in "${@:2}"; do
    case ${script_arg} in
        "--test")
            test_containers
            ;;
        "--publish")
            echo "Publishing..."
            publish_containers
            ;;
        *)
            echo "Error: Unrecognized argument '${script_arg}'"
            break
            ;;
    esac
done
