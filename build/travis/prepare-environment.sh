# Travis sources this script, so we can export variables into the
# outer shell, so we don't want to set options like nounset because
# they would be set in the outer shell as well, so do as much logic as
# we can in a subshell.
(
    set -o nounset -o errexit -o pipefail

    if [ "${TRAVIS_OS_NAME:-}" = "osx" ]; then
        sudo mkdir /opt/pulumi
        sudo chown "${USER}" /opt/pulumi
    fi

    # We have some shared scripts in pulumi/home, and we use them in other steps
    git clone git@github.com:pulumi/home "$(go env GOPATH)/src/github.com/pulumi/home"

    # If we have an NPM token, put it in the .npmrc file, so we can use it:
    if [ ! -z "${NPM_TOKEN:-}" ]; then
        echo "//registry.npmjs.org/:_authToken=\${NPM_TOKEN}" > ~/.npmrc
    fi
)

# If the sub shell failed, bail out now.
[ "$?" -eq 0 ] || exit 1

export PULUMI_ROOT=/opt/pulumi
export PULUMI_FAILED_TESTS_DIR=$(mktemp -d)
echo "PULUMI_FAILED_TESTS_DIR=${PULUMI_FAILED_TESTS_DIR}"
