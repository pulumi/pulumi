nvm install ${NODE_VERSION-v8.11.1}

# Travis sources this script, so we can export variables into the
# outer shell, so we don't want to set options like nounset because
# they would be set in the outer shell as well, so do as much logic as
# we can in a subshell.
#
# We do need "OS" to be set in the parent shell, though, since we
# inspect it at the end of the script in order to determine whether or
# not to munge the PATH to include Python executables.
export OS=""
case $(uname) in
    "Linux") OS="linux";;
    "Darwin") OS="darwin";;
    *) echo "error: unknown host os $(uname)" ; exit 1;;
esac

(
    set -o nounset -o errexit -o pipefail
    [ -e "$(go env GOPATH)/bin" ] || mkdir -p "$(go env GOPATH)/bin"

    YARN_VERSION="${YARN_VERSION:-1.13.0}"
    DEP_VERSION="${DEP_VERSION:-0.5.0}"
    GOLANGCI_LINT_VERSION="${GOLANGCI_LINT_VERSION:-1.16.0}"
    PIP_VERSION="${PIP_VERSION:-10.0.0}"
    VIRTUALENV_VERSION="${VIRTUALENV_VERSION:-15.2.0}"
    PIPENV_VERSION="${PIPENV_VERSION:-2018.10.13}"
    AWSCLI_VERSION="${AWSCLI_VERSION:-1.14.30}"
    WHEEL_VERSION="${WHEEL_VERSION:-0.30.0}"
    TWINE_VERSION="${TWINE_VERSION:-1.9.1}"
    TF2PULUMI_VERSION="${PULUMI_VERSION:-0.5.0}"
    PANDOC_VERSION="${PANDOC_VERSION:-2.6}"

    # jq isn't present on OSX, but we use it in some of our scripts. Install it.
    if [ "${OS}" = "darwin" ]; then
        brew update
        brew install jq
    fi

    echo "installing yarn ${YARN_VERSION}"
    curl -o- -L https://yarnpkg.com/install.sh | bash -s -- --version ${YARN_VERSION}

    echo "installing dep ${DEP_VERSION}"
    curl -L -o "$(go env GOPATH)/bin/dep" https://github.com/golang/dep/releases/download/v${DEP_VERSION}/dep-${OS}-amd64
    chmod +x "$(go env GOPATH)/bin/dep"

    echo "installing GolangCI-Lint ${GOLANGCI_LINT_VERSION}"
    curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b "$(go env GOPATH)/bin" "v${GOLANGCI_LINT_VERSION}"

    echo "installing gocovmerge"

    # gocovmerge does not publish versioned releases, but it also hasn't been updated in two years, so
    # getting HEAD is pretty safe.
    go get -v github.com/wadey/gocovmerge

    echo "upgrading Pip to ${PIP_VERSION}"
    sudo pip install --upgrade "pip>=${PIP_VERSION}"
    pip install --user --upgrade "pip>=${PIP_VERSION}"

    echo "installing virtualenv ${VIRTUALENV_VERSION}"
    sudo pip install "virtualenv==${VIRTUALENV_VERSION}"
    pip install --user "virtualenv==${VIRTUALENV_VERSION}"

    echo "installing pipenv ${PIPENV_VERSION}"
    pip install --user "pipenv==${PIPENV_VERSION}"

    echo "installing AWS cli ${AWSCLI_VERSION}"
    pip install --user "awscli==${AWSCLI_VERSION}"

    echo "installing Wheel and Twine, so we can publish Python packages"
    pip install --user "wheel==${WHEEL_VERSION}" "twine==${TWINE_VERSION}"

    echo "installing pandoc, so we can generate README.rst for Python packages"
    if [ "${OS}" = "linux" ]; then
        curl -sfL -o /tmp/pandoc.deb "https://github.com/jgm/pandoc/releases/download/${PANDOC_VERSION}/pandoc-${PANDOC_VERSION}-1-amd64.deb"
        sudo apt-get install /tmp/pandoc.deb
    else
        # This is currently version 2.6 - we'll likely want to track the version
        # in brew pretty closely in CI, as it's a pain to install otherwise.
        brew install pandoc
    fi

    echo "installing dotnet sdk and runtime"
    if [ "${OS}" = "linux" ]; then
        wget -q https://packages.microsoft.com/config/ubuntu/18.04/packages-microsoft-prod.deb -O packages-microsoft-prod.deb
        sudo dpkg -i packages-microsoft-prod.deb
        sudo add-apt-repository universe
        sudo apt-get update
        sudo apt-get install apt-transport-https
        sudo apt-get update
        sudo apt-get install dotnet-sdk-3.0
        sudo apt-get install aspnetcore-runtime-3.0
    else
        brew cask install dotnet-sdk
        brew cask install dotnet
    fi

    echo "installing Terraform-to-Pulumi conversion tool (${TF2PULUMI_VERSION}-${OS})"
    curl -L "https://github.com/pulumi/tf2pulumi/releases/download/v${TF2PULUMI_VERSION}/tf2pulumi-v${TF2PULUMI_VERSION}-${OS}-x64.tar.gz" | \
			tar -xvz -C "$(go env GOPATH)/bin"

    echo "installing gomod-doccopy"
    go get -v github.com/pulumi/scripts/gomod-doccopy
)

# If the sub shell failed, bail out now.
[ "$?" -eq 0 ] || exit 1

# By default some tools are not on the PATH, let's fix that

# On OSX, the location that pip installs helper scripts to isn't on the path
if [ "${OS}" = "darwin" ]; then
    export PATH=$PATH:$HOME/Library/Python/2.7/bin
fi

# Add yarn to the $PATH
export PATH=$HOME/.yarn/bin:$PATH
