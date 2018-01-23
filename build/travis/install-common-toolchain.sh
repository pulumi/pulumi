nvm install v6.10.2

# Travis sources this script, so we can export variables into the
# outer shell, so we don't want to set options like nounset because
# they would be set in the outer shell as well, so do as much logic as
# we can in a subshell.
(
    set -o nounset -o errexit -o pipefail
    [ -e "$(go env GOPATH)/bin" ] || mkdir -p "$(go env GOPATH)/bin"

    YARN_VERSION="1.3.2"
    DEP_VERSION="0.4.1"
    GOMETALINTER_VERSION="2.0.3"
    AWSCLI_VERSION="1.14.30"

    OS=""
    case $(uname) in
        "Linux") OS="linux";;
        "Darwin") OS="darwin";;
        *) echo "error: unknown host os $(uname)" ; exit 1;;
    esac

    PIP_CMD=pip

    # On Travis, pip is called pip2.7
    if [ "${TRAVIS_OS_NAME:-}" = "osx" ]; then
        PIP_CMD=pip2.7
    fi

    echo "installing yarn ${YARN_VERSION}"
    curl -o- -L https://yarnpkg.com/install.sh | bash -s -- --version ${YARN_VERSION}

    echo "installing dep ${DEP_VERSION}"
    curl -L -o "$(go env GOPATH)/bin/dep" https://github.com/golang/dep/releases/download/v${DEP_VERSION}/dep-${OS}-amd64
    chmod +x "$(go env GOPATH)/bin/dep"


    echo "installing Gometalinter ${GOMETALINTER_VERSION}"
    curl -L "https://github.com/alecthomas/gometalinter/releases/download/v${GOMETALINTER_VERSION}/gometalinter-v${GOMETALINTER_VERSION}-${OS}-amd64.tar.bz2" | tar -jxv --strip-components=1 -C "$(go env GOPATH)/bin"

    chmod +x "$(go env GOPATH)/bin/gometalinter"
    chmod +x "$(go env GOPATH)/bin/linters/"*

    # Gometalinter looks for linters on the $PATH, so let's move them out
    # of the linters folder and into GOBIN (which we know is on the $PATH)
    mv "$(go env GOPATH)/bin/linters/"* "$(go env GOPATH)/bin/."
    rm -rf "$(go env GOPATH)/bin/linters/"

    echo "installing gocovmerge"

    # gocovmerge does not publish versioned releases, but it also hasn't been updated in two years, so
    # getting HEAD is pretty safe.
    go get -v github.com/wadey/gocovmerge

    echo "installing AWS cli ${AWSCLI_VERSION}"
    ${PIP_CMD} install --user "awscli==${AWSCLI_VERSION}"
)

# If the sub shell failed, bail out now.
[ "$?" -eq 0 ] || exit 1

# By default some tools are not on the PATH, let's fix that

# On OSX, the user folder that `pip` installs tools to is not on the
# $PATH by default.
if [[ "${TRAVIS_OS_NAME:-}" == "osx" ]]; then
    export PATH=$PATH:$HOME/Library/Python/2.7/bin
fi

# Add yarn to the $PATH
export PATH=$HOME/.yarn/bin:$PATH
