# Contributing to Pulumi

First, thanks for contributing to Pulumi and helping make it better. We appreciate the help! If you're looking for an issue to start with, we've tagged some issues with the [help-wanted](https://github.com/pulumi/pulumi/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22) tag but feel free to pick up any issue that looks interesting to you or fix a bug you stumble across in the course of using Pulumi. No matter the size, we welcome all improvements.

For larger features, we'd appreciate it if you open a [new issue](https://github.com/pulumi/pulumi/issues/new) before doing a ton of work to discuss the feature before you start writing a lot of code.

## Hacking on Pulumi

To hack on Pulumi, you'll need to get a development environment set up. You'll want to install the following on your machine:

- Go 1.16
- NodeJS 10.X.X or later
- Python 3.6 or later
- [.NET Core](https://dotnet.microsoft.com/download)
- [pipenv](https://github.com/pypa/pipenv)
- [Golangci-lint](https://github.com/golangci/golangci-lint)
- [Yarn](https://yarnpkg.com/)

## Getting dependencies on macOS

You can easily get all required dependencies with brew and npm

```bash
brew install node pipenv python@3 typescript yarn go@1.13 golangci/tap/golangci-lint pulumi/tap/pulumictl
curl https://raw.githubusercontent.com/Homebrew/homebrew-cask/0272f0d33f/Casks/dotnet-sdk.rb > dotnet-sdk.rb  # v3.1.0
brew install --HEAD -s dotnet-sdk.rb
rm dotnet-sdk.rb
```

## Hacking on Pulumi in Gitpod

If you have a web browser, you can get a fully pre-configured Pulumi development environment in one click:

[![Open in Gitpod](https://gitpod.io/button/open-in-gitpod.svg)](https://gitpod.io/#https://github.com/pulumi/pulumi)

## Make build system

We use `make` as our build system, so you'll want to install that as well, if you don't have it already. We have extremely limited support for doing development on Windows (the bare minimum for us to get Windows validation of `pulumi`) so if you're on windows, we recommend that you use the [Windows Subsystem for Linux](https://docs.microsoft.com/en-us/windows/wsl/install-win10). We'd like to [make this better](https://github.com/pulumi/pulumi/issues/208) so feel free to pitch in if you can.

For historical reasons (which we'd [like to address](https://github.com/pulumi/pulumi/issues/1515)) our build system requires that the folder `/opt/pulumi` exists and is writable by the current user. If you'd like, you can override this location by setting `PULUMI_ROOT` in your environment. The build is known to fail if this doesn't exist, so you'll need to create it first.

```bash
mkdir /opt/pulumi
sudo chown <your_user_name>: /opt/pulumi
export PATH=/opt/pulumi:/opt/pulumi/bin:$PATH
```

You'll also need to make sure your maximum open file descriptor limit is set to 5000 at a minimum.

```bash
ulimit -n # to test
ulimit -n 5000
```

Across our projects, we try to use a regular set of make targets. The ones you'll care most about are:

0. `make ensure`, which restores/installs any build dependencies
1. `make`, which builds Pulumi and runs a quick set of tests
2. `make all` which builds Pulumi and runs the quick tests and a larger set of tests.

We make heavy use of integration level testing where we invoke `pulumi` to create and then delete cloud resources. This requires you to have a Pulumi account (so [sign up for free](https://pulumi.com) today if you haven't already) and login with `pulumi login`.

This repository does not actually create any real cloud resources as part of testing, but still uses Pulumi.com to store information abot some synthetic resources it creates during testing. Other repositories may require additional setup before running tests (most often this is just setting a few environment variables that tell the tests some information about how to use the cloud provider we are testing). Please see the `CONTRIBUTING.md` file in the repository, which will explain what additional configuration needs to be done before running tests.

## Debugging

The Pulumi tools have extensive logging built in.  In fact, we encourage liberal logging in new code, and adding new logging when debugging problems.  This helps to ensure future debugging endeavors benefit from your sleuthing.

All logging is done using a fork of Google's [Glog library](https://github.com/pulumi/glog).  It is relatively bare-bones, and adds basic leveled logging, stack dumping, and other capabilities beyond what Go's built-in logging routines offer.

The `pulumi` command line has two flags that control this logging and that can come in handy when debugging problems. The `--logtostderr` flag spews directly to stderr, rather than the default of logging to files in your temp directory. And the `--verbose=n` flag (`-v=n` for short) sets the logging level to `n`.  Anything greater than 3 is reserved for debug-level logging, greater than 5 is going to be quite verbose, and anything beyond 7 is extremely noisy.

For example, the command

```sh
$ pulumi preview --logtostderr -v=5
```

is a pretty standard starting point during debugging that will show a fairly comprehensive trace log of a compilation.

## Submitting a Pull Request

For contributors we use the standard fork based workflow. Fork this repository, create a topic branch, and start hacking away.  When you're ready, make sure you've run the tests (`make travis_pull_request` will run the exact flow we run in CI) and open your PR.
When adding a changelog entry, please be sure to use `CHANGELOG_PENDING.md` for the entry - we will then be able to ensure your PR gets into the next release.

## Getting Help

We're sure there are rough edges and we appreciate you helping out. If you want
to talk with other folks hacking on Pulumi (or members of the Pulumi team!)
come hang out `#contribute` channel in the
[Pulumi Community Slack](https://slack.pulumi.com/).
