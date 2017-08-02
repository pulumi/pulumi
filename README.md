[![Build Status](https://travis-ci.com/pulumi/lumi.svg?token=cTUUEgrxaTEGyecqJpDn&branch=master)](https://travis-ci.com/pulumi/lumi)

# Lumi

Lumi is a framework and toolset for creating reusable cloud services.

If you are learning about Lumi for the first time, please see [the overview document](docs/overview.md).

## Installing

To install Lumi from source, simply run:

    $ go get -u github.com/pulumi/lumi/cmd/lumi

A `GOPATH` must be set.  A good default value is `~/go`.  In fact, [this is the default in Go 1.8](
https://github.com/golang/go/issues/17262).

This installs the `lumi` binary to `$GOPATH/bin`.

At this moment, libraries must be manually installed.  See below.  Eventually we will have an installer.

## Compilers

The Lumi compilers are independent from the core Lumi tools.

Please see the respective pages for details on how to install, build, and test each compiler:

* [LumiJS](cmd/lumijs/README.md)
* [LumiPy](cmd/lumipy/README.md)

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

    $ git clone git@github.com:pulumi/lumi $GOPATH/src/github.com/pulumi/lumi
    $ cd $GOPATH/src/github.com/pulumi/lumi

Before building, you will need to ensure dependencies have been restored to your enlistment:

    $ dep ensure

At this point you can run `make` to build and run tests:

    $ make

This installs the `lumi` binary into `$GOPATH/bin`, which may now be run provided `make` exited successfully.

The Makefile also supports just running tests (`make test`), just running the linter (`make lint`), just running Govet
(`make vet`), and so on.  Please just refer to the Makefile for the full list of targets.

### Installing the Runtime Libraries

By default, Lumi looks for its runtime libraries underneath `/usr/local/lumi`.  `$LUMIPATH` overrides this.
Please refer to the [libraries README](lib/README.md) for details on additional installation requirements.

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

