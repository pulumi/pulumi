[![Build Status](https://travis-ci.com/pulumi/pulumi-fabric.svg?token=cTUUEgrxaTEGyecqJpDn&branch=master)](https://travis-ci.com/pulumi/pulumi-fabric)

# Pulumi Fabric

The Pulumi Fabric ("Lumi") is a framework and toolset for creating reusable cloud services.

If you are learning about Lumi for the first time, please see [the overview document](docs/overview.md).

## Installing

To install Lumi from source, simply run:

    $ go get -u github.com/pulumi/pulumi-fabric/cmd/lumi

A `GOPATH` must be set.  A good default value is `~/go`.  In fact, [this is the default in Go 1.8](
https://github.com/golang/go/issues/17262).

This installs the `lumi` binary to `$GOPATH/bin`.

To do anything interesting with Lumi, you will need an SDK for your language of choice.  Please see
[sdk/README.md](`sdk/`) for information about how to obtain, install, and use such an SDK.

## Development

This section is for Lumi developers.

### Prerequisites

Lumi is written in Go, uses Dep for dependency management, and GoMetaLinter for linting:

* [Go](https://golang.org/doc/install): https://golang.org/dl
* [Dep](https://github.com/golang/dep): `$ go get -u github.com/golang/dep/cmd/dep`
* [GoMetaLinter](https://github.com/alecthomas/gometalinter):
    - `$ go get -u github.com/alecthomas/gometalinter`
    - `$ gometalinter --install`

### Building and Testing

To build Lumi, ensure `$GOPATH` is set, and clone into a standard Go workspace:

    $ git clone git@github.com:pulumi/pulumi-fabric $GOPATH/src/github.com/pulumi/pulumi-fabric
    $ cd $GOPATH/src/github.com/pulumi/pulumi-fabric

Before building, you will need to ensure dependencies have been restored to your enlistment:

    $ dep ensure

At this point you can run `make` to build and run tests:

    $ make

This installs the `lumi` binary into `$GOPATH/bin`, which may now be run provided `make` exited successfully.

The Makefile also supports just running tests (`make test`), just running the linter (`make lint`), just running Govet
(`make vet`), and so on.  Please just refer to the Makefile for the full list of targets.

### Debugging

The Lumi tools have extensive logging built in.  In fact, we encourage liberal logging in new code, and adding new
logging when debugging problems.  This helps to ensure future debugging endeavors benefit from your sleuthing.

All logging is done using Google's [Glog library](https://github.com/golang/glog).  It is relatively bare-bones, and
adds basic leveled logging, stack dumping, and other capabilities beyond what Go's built-in logging routines offer.

The Lumi command line has two flags that control this logging and that can come in handy when debugging problems.  The
`--logtostderr` flag spews directly to stderr, rather than the default of logging to files in your temp directory.  And
the `--verbose=n` flag (`-v=n` for short) sets the logging level to `n`.  Anything greater than 3 is reserved for
debug-level logging, greater than 5 is going to be quite verbose, and anything beyond 7 is extremely noisy.

For example, the command

    $ lumi eval --logtostderr -v=5

is a pretty standard starting point during debugging that will show a fairly comprehensive trace log of a compilation.

