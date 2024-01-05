# Contributing to Pulumi

First, thanks for contributing to Pulumi and helping make it better. We appreciate the help!
This repository is one of many across the Pulumi ecosystem and we welcome contributions to them all.

## Code of Conduct

Please make sure to read and observe our [Contributor Code of Conduct](./CODE-OF-CONDUCT.md).

## Communications

You are welcome to join the [Pulumi Community Slack](https://slack.pulumi.com/) for questions and a community of like-minded folks.
We discuss features and file bugs on GitHub via [Issues](https://github.com/pulumi/pulumi/issues) as well as [Discussions](https://github.com/pulumi/pulumi/discussions).
You can read about our [public roadmap](https://github.com/orgs/pulumi/projects/44) on the [Pulumi blog](https://www.pulumi.com/blog/relaunching-pulumis-public-roadmap/).

### Issues

Feel free to pick up any existing issue that looks interesting to you or fix a bug you stumble across while using Pulumi. No matter the size, we welcome all improvements.

### Feature Work

For larger features, we'd appreciate it if you open a [new issue](https://github.com/pulumi/pulumi/issues/new) before investing a lot of time so we can discuss the feature together.
Please also be sure to browse [current issues](https://github.com/pulumi/pulumi/issues) to make sure your issue is unique, to lighten the triage burden on our maintainers.
Finally, please limit your pull requests to contain only one feature at a time. Separating feature work into individual pull requests helps speed up code review and reduces the barrier to merge.

## Developing

### Setting up your Pulumi development environment

You'll want to install the following on your machine:

- [Go](https://go.dev/dl/) (a [supported version](https://go.dev/doc/devel/release#policy))
- [NodeJS 16.X.X or later](https://nodejs.org/en/download/)
- [Python 3.6 or later](https://www.python.org/downloads/)
- [.NET](https://dotnet.microsoft.com/download)
- [Golangci-lint](https://github.com/golangci/golangci-lint)
- [gofumpt](https://github.com/mvdan/gofumpt):
  see [installation](https://github.com/mvdan/gofumpt#installation) for editor setup instructions
- [Yarn](https://yarnpkg.com/)
- [Pulumictl](https://github.com/pulumi/pulumictl)
- [jq](https://stedolan.github.io/jq/)

### Installing Pulumi dependencies on macOS

You can get all required dependencies with brew and npm

```bash
brew install node python@3 typescript yarn go@1.21 golangci/tap/golangci-lint gofumpt pulumi/tap/pulumictl coreutils jq
curl https://raw.githubusercontent.com/Homebrew/homebrew-cask/339862f79e/Casks/dotnet-sdk.rb > dotnet-sdk.rb
brew install --HEAD -s dotnet-sdk.rb
rm dotnet-sdk.rb
```

### Working on Pulumi in Gitpod

If you have a web browser, you can get a fully pre-configured Pulumi development environment in one click:

[![Open in Gitpod](https://gitpod.io/button/open-in-gitpod.svg)](https://gitpod.io/#https://github.com/pulumi/pulumi)

### Make build system

We use `make` as our build system, so you'll want to install that as well, if you don't have it already. We have extremely limited support for doing development on Windows (the bare minimum for us to get Windows validation of `pulumi`) so if you're on windows, we recommend that you use the [Windows Subsystem for Linux](https://docs.microsoft.com/en-us/windows/wsl/install-win10). We'd like to [make this better](https://github.com/pulumi/pulumi/issues/208) so feel free to pitch in if you can.

We build Pulumi in `$PULUMI_ROOT`, which defaults to `$HOME/.pulumi-dev`. If you would like to build Pulumi in another location, you do so by setting `$PULUMI_ROOT`.

```bash
export PATH=$HOME/.pulumi-dev/bin:$PATH
```

You'll also need to make sure your maximum open file descriptor limit is set to 5000 at a minimum.

```bash
ulimit -n # to test
ulimit -n 5000
```

Across our projects, we try to use a regular set of make targets. The ones you'll care most about are:

1. `make ensure`, which restores/installs any build dependencies
1. `make dist`, which just builds and installs the Pulumi CLI
1. `make`, which builds Pulumi and runs a quick set of tests
1. `make all` which builds Pulumi and runs the quick tests and a larger set of tests.

We make heavy use of integration level tests that invoke `pulumi` to create and then delete cloud resources. In order to run our integration tests, you will need a Pulumi account (so [sign up for free](https://pulumi.com) today if you haven't already) and log in with `pulumi login`.  Additionally, before running integration tests, be sure to set the environment variable `PULUMI_TEST_ORG` to your pulumi username.

The tests in this repository do not create any real cloud resources as part of testing but still uses Pulumi.com to store information about some synthetic resources the tests create. Other repositories may require additional setup before running tests. In most cases, this additional setup consists of setting a few environment variables to configure the provider for the the cloud service we are testing. Please see the `CONTRIBUTING.md` file in the relevant repository, which will explain what additional configuration is needed before running tests.

### Regenerate Test Baselines

Numerous tests use baselines that need to be regenerated from time to time. For instance, `pkg/backend/display/testdata` contains the corresponding CLI output for various engine event streams. To regenerate these baselines, run the corresponding test with the `PULUMI_ACCEPT=true` environment variable. For instance, `PULUMI_ACCEPT=true make test_all` from the root. Alternatively, you can generate them individually, for example, running `PULUMI_ACCEPT=true go test ./...` from the `pkg/backend/display` directory.

### Debugging

The Pulumi tools have extensive logging built in.  In fact, we encourage liberal logging in new code, and adding new logging when debugging problems.  This helps to ensure future debugging endeavors benefit from your sleuthing.

All logging is done using a fork of Google's [Glog library](https://github.com/pulumi/glog).  It is relatively bare-bones, and adds basic leveled logging, stack dumping, and other capabilities beyond what Go's built-in logging routines offer.

The `pulumi` command line has two flags that control this logging and that can come in handy when debugging problems. The `--logtostderr` flag spews directly to stderr, rather than the default of logging to files in your temp directory. And the `--verbose=n` flag (`-v=n` for short) sets the logging level to `n`.  Anything greater than 3 is reserved for debug-level logging, greater than 5 is going to be quite verbose, and anything beyond 7 is extremely noisy.

For example, the command

```sh
$ pulumi preview --logtostderr -v=5
```

is a pretty standard starting point during debugging that will show a fairly comprehensive trace log of a compilation.

### Go Language Server

Since this repository contains multiple go modules, `gopls` requires a go workspace. Run `make work` to setup a suitable go workspace.

## Submitting a Pull Request

For contributors we use the [standard fork based workflow](https://gist.github.com/Chaser324/ce0505fbed06b947d962): Fork this repository, create a topic branch, and when ready, open a pull request from your fork.

Before you open a pull request, make sure all lint checks pass:

```bash
$ make lint
```

If you see formatting failures, fix them by running [gofumpt](https://github.com/mvdan/gofumpt) on your code:

```bash
$ gofumpt -w path/to/file.go 
# or
$ gofumpt -w path/to/dir
```

We require a changelog entry for all PR that aren't labeled `impact/no-changelog-required`. To generate a new changelog entry, run…

```bash
$ make changelog
````
…and follow the prompts on screen.

### Pulumi employees

Pulumi employees have write access to Pulumi repositories and should push directly to branches rather than forking the repository. Tests can run directly without approval for PRs based on branches rather than forks.

Please ensure that you nest your branches under a unique identifier such as your name (e.g. `refs/heads/pulumipus/cool_feature`).

## Getting Help

We're sure there are rough edges and we appreciate you helping out. If you want to talk with other folks in the Pulumi community (including members of the Pulumi team) come hang out in the `#contribute` channel on the [Pulumi Community Slack](https://slack.pulumi.com/).
