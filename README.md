# Pulumi

Pulumi is a cloud development platform that makes creating cloud programs easy and productive.

Author cloud programs in your favorite language and Pulumi will automatically keep your
infrastructure up-to-date.  Skip the YAML and just write code.  Pulumi is multi-language, multi-cloud
and fully extensible in both its engine and ecosystem of packages.

To install the latest Pulumi release, run:

```bash
$ curl -fsSL https://get.pulumi.com/ | sh
```

After installing, you can get started with the `pulumi new` command,
[our examples](https://github.com/pulumi/examples), or our [visit project website](https://pulumi.io/) which
includes several [in-depth tutorials](https://pulumi.io/quickstart) and
[an interactive tour](https://pulumi.io/tour) to walk through the core CLI usage and programming concepts.

Please join [the conversation on Slack](https://slack.pulumi.io/).

This repo contains the CLI, language SDKs, and the core Pulumi engine.  Individual libraries are in their own repos.

## Platforms

| Architecture | Build Status |
| ------------ | ------------ |
| Linux/macOS x64    | [![Linux x64 Build Status](https://travis-ci.com/pulumi/pulumi.svg?token=cTUUEgrxaTEGyecqJpDn&branch=master)](https://travis-ci.com/pulumi/pulumi)  |
| Windows x64  | [![Windows x64 Build Status](https://ci.appveyor.com/api/projects/status/uqrduw6qnoss7g4i?svg=true&branch=master)](https://ci.appveyor.com/project/pulumi/pulumi) |

## Languages

| Language | Status | Runtime | Readme |
| -------- | ------ | ------- | -------- |
| JavaScript | Stable | Node.js 6.x-10.x | [Readme](./sdk/nodejs) |
| TypeScript | Stable | Node.js 6.x-10.x | [Readme](./sdk/nodejs) |
| Python | Preview | Python 2.7 | [Readme](./sdk/python) |
| Go | Preview | Go 1.x | [Readme](./sdk/go) |

## Clouds

| Cloud | Status | Docs | Repo |
| ----- | ------ | ---- | ---- |
| Amazon Web Services | Stable | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/aws/) | [pulumi/pulumi-aws](https://github.com/pulumi/pulumi-aws) |
| Microsoft Azure | Preview | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/azure/) | [pulumi/pulumi-azure](https://github.com/pulumi/pulumi-azure) |
| Google Cloud Platform | Preview | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/gcp/) | [pulumi/pulumi-gcp](https://github.com/pulumi/pulumi-gcp) |
| Kubernetes | Preview | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/kubernetes/) | [pulumi/pulumi-kubernetes](https://github.com/pulumi/pulumi-kubernetes) |

## Libraries

There are several libraries that encapsulate best practices and common patterns:

| Library | Status | Docs | Repo |
| ------- | ------ | ---- | ---- |
| AWS Serverless | Preview | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/aws-serverless/) | [pulumi/pulumi-aws-serverless](https://github.com/pulumi/pulumi-aws-serverless) |
| AWS Infrastructure | Preview | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/aws-infra/) | [pulumi/pulumi-aws-infra](https://github.com/pulumi/pulumi-aws-infra) |
| Pulumi Multi-Cloud Framework | Preview | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/cloud/) | [pulumi/pulumi-cloud](https://github.com/pulumi/pulumi-cloud) |

## Examples

A collection of examples for different languages, clouds, and scenarios is available in the
[pulumi/examples](https://github.com/pulumi/examples) repo.

## Development

If you'd like to contribute to Pulumi and/or build from source, this section is for you.

### Prerequisites

Pulumi is written in Go, uses Dep for dependency management, and GoMetaLinter for linting:

* [Go](https://golang.org/doc/install): https://golang.org/dl
* [Dep](https://github.com/golang/dep): `$ go get -u github.com/golang/dep/cmd/dep`
* [GoMetaLinter](https://github.com/alecthomas/gometalinter):
    - `$ go get -u github.com/alecthomas/gometalinter`
    - `$ gometalinter --install`

### Building and Testing

To install the pre-built SDK, please run `curl -fsSL https://get.pulumi.com/ | sh`, or see detailed installation instructions on [the project page](https://pulumi.io/).  Read on if you want to install from source.

To build a complete Pulumi SDK, ensure `$GOPATH` is set, and clone into a standard Go workspace:

    $ git clone git@github.com:pulumi/pulumi $GOPATH/src/github.com/pulumi/pulumi
    $ cd $GOPATH/src/github.com/pulumi/pulumi

The first time you build, you must `make ensure` to install dependencies and perform other machine setup:

    $ make ensure

In the future, you can synch dependencies simply by running `dep ensure` explicitly:

    $ dep ensure

At this point you can run `make` to build and run tests:

    $ make

This installs the `pulumi` binary into `$GOPATH/bin`, which may now be run provided `make` exited successfully.

The Makefile also supports just running tests (`make test_all` or `make test_fast`), just running the linter
(`make lint`), just running Govet (`make vet`), and so on.  Please just refer to the Makefile for the full list of targets.

### Debugging

The Pulumi tools have extensive logging built in.  In fact, we encourage liberal logging in new code, and adding new
logging when debugging problems.  This helps to ensure future debugging endeavors benefit from your sleuthing.

All logging is done using Google's [Glog library](https://github.com/golang/glog).  It is relatively bare-bones, and
adds basic leveled logging, stack dumping, and other capabilities beyond what Go's built-in logging routines offer.

The `pulumi` command line has two flags that control this logging and that can come in handy when debugging problems.
The `--logtostderr` flag spews directly to stderr, rather than the default of logging to files in your temp directory.
And the `--verbose=n` flag (`-v=n` for short) sets the logging level to `n`.  Anything greater than 3 is reserved for
debug-level logging, greater than 5 is going to be quite verbose, and anything beyond 7 is extremely noisy.

For example, the command

    $ pulumi preview --logtostderr -v=5

is a pretty standard starting point during debugging that will show a fairly comprehensive trace log of a compilation.
