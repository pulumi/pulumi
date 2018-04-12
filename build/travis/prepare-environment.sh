# Travis sources this script, so we can export variables into the
# outer shell, so we don't want to set options like nounset because
# they would be set in the outer shell as well, so do as much logic as
# we can in a subshell.

export PULUMI_HOME="$(go env GOPATH)/src/github.com/pulumi/home"
export PULUMI_SDK="$(go env GOPATH)/src/github.com/pulumi/sdk"

(
    set -o nounset -o errexit -o pipefail

    if [ "${TRAVIS_OS_NAME:-}" = "osx" ]; then
        sudo mkdir /opt/pulumi
        sudo chown "${USER}" /opt/pulumi
    fi

    # We have some shared scripts in pulumi/home, and we use them in other steps
    git clone git@github.com:pulumi/home "${PULUMI_HOME}"

    # We have some shared scripts in pulumi/sdk, and we use them in other steps
    git clone git@github.com:pulumi/sdk "${PULUMI_SDK}"

    # If we have an NPM token, put it in the .npmrc file, so we can use it:
    if [ ! -z "${NPM_TOKEN:-}" ]; then
        echo "//registry.npmjs.org/:_authToken=\${NPM_TOKEN}" > ~/.npmrc
    fi
) || exit 1  # Abort outer script if subshell fails.

export PULUMI_ROOT=/opt/pulumi
