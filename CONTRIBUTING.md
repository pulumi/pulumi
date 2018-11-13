# Contributing to Pulumi

First, thanks for contributing to Pulumi and helping make it better. We appreciate the help! If you're looking for an issue to start with, we've tagged some issues with the [help-wanted](https://github.com/pulumi/pulumi/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22) tag but feel free to pick up any issue that looks interesting to you or fix a bug you stumble across in the course of using Pulumi. No matter the size, we welcome all improvements.

For larger features, we'd appreciate it if you open a [new issue](https://github.com/pulumi/pulumi/issues/new) before doing a ton of work to discuss the feature before you start writing a lot of code.

## Hacking on Pulumi

To hack on Pulumi, you'll need to get a development environment set up. You'll want to install the following on your machine:

- Go 1.9 or later
- NodeJS 6.10.X or later
- Python 3.6 or later
- [pipenv](https://github.com/pypa/pipenv)
- [dep](https://github.com/golang/dep)
- [Golangci-lint](https://github.com/golangci/golangci-lint)
- [Yarn](https://yarnpkg.com/)

## Getting dependencies on macOS

You can easily get all required dependencies with brew

```bash
brew install node pipenv python@3 typescript dep yarn pandoc
```

## Make build system

We use `make` as our build system, so you'll want to install that as well, if you don't have it already. We have extremely limited support for doing development on Windows (the bare minimum for us to get Windows validation of `pulumi`) so if you're on windows, we recommend that you use the [Windows Subsystem for Linux](https://docs.microsoft.com/en-us/windows/wsl/install-win10). We'd like to [make this better](https://github.com/pulumi/pulumi/issues/208) so feel free to pitch in if you can.

For historical reasons (which we'd [like to address](https://github.com/pulumi/pulumi/issues/1515)) our build system requires that the folder `/opt/pulumi` exists and is writable by the current user. If you'd like, you can override this location by setting `PULUMI_ROOT` in your environment. The build is known to fail if this doesn't exist, so you'll need to create it first.

Across our projects, we try to use a regular set of make targets. The ones you'll care most about are:

1. `make`, which builds Pulumi and runs a quick set of tests
2. `make all` which builds Pulumi and runs the quick tests and a larger set of tests.

We make heavy use of integration level testing where we invoke `pulumi` to create and then delete cloud resources. This requires you to have a Pulumi account (so [sign up for free](https://pulumi.com) today if you haven't already) and login with `pulumi login`.

This repository does not actually create any real cloud resources as part of testing, but still uses Pulumi.com to store information abot some synthetic resources it creates during testing. Other repositories may require additional setup before running tests (most often this is just setting a few environment variables that tell the tests some information about how to use the cloud provider we are testing). Please see the `CONTRIBUTING.md` file in the repository, which will explain what additional configuration needs to be done before running tests.

Pulumi integration tests make use of the Go test runner. When using Go 1.10 or above, we recommend setting the `GOCACHE` environment variable to `off` to avoid
erroneously caching test results.

## Submitting a Pull Request

For contributors we use the standard fork based workflow. Fork this repository, create a topic branch, and start hacking away.  When you're ready, make sure you've run the tests (`make travis_pull_request` will run the exact flow we run in CI) and open your PR.

## Getting Help

We're sure there are rough edges and we appreciate you helping out. If you want to talk with other folks hacking on Pulumi (or members of the Pulumi team!) come hang out `#contribute` channel in the [Pulumi Community Slack](https://slack.pulumi.io/).
