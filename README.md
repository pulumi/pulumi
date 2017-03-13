# Coconut

Coconut is a framework and toolset for creating reusable cloud services.

If you are learning about Coconut for the first time, please see [the overview document](docs/overview.md).

## Installing

To install Coconut from source, simply run:

    $ go get -u github.com/pulumi/coconut

A `GOPATH` must be set.  A good default value is `~/go`.  In fact, [this is the default in Go 1.8](
https://github.com/golang/go/issues/17262).

It is common to alias the shorter command `coco` to the full binary `coconut`:

    alias coco=coconut

At this moment, libraries must be manually installed.  See below.  Eventually we will have an installer.

## Compilers

The Coconut compilers are independent from the core Coconut tools.

Please see the respective pages for details on how to install, build, and test each compiler:

* [CoconutJS](tools/cocojs/README.md)

## Development

This section is for Coconut developers.

### Prerequisites

Coconut is written in Go and uses Glide for dependency management.  They must be installed:

* [Go](https://golang.org/doc/install)
* [Glide](https://github.com/Masterminds/glide)

If you wish to use the optional `lint` make target, you'll also need to install Golint:

    $ go get -u github.com/golang/lint/golint

### Building and Testing

To build Coconut, ensure `$GOPATH` is set, and clone into a standard Go workspace:

    $ git clone git@github.com:pulumi/coconut $GOPATH/src/github.com/pulumi/coconut

At this point you should be able to build and run tests from the root directory:

    $ cd $GOPATH/src/github.com/pulumi/coconut
    $ glide update
    $ make

This installs the `coconut` binary into `$GOPATH/bin`, which may now be run provided `make` exited successfully.

### Installing the Runtime Libraries

By default, Coconut looks for its runtime libraries underneath `/usr/local/coconut`.  `$COCOPATH` overrides this.
Please refer to the [libraries README](lib/README.md) for details on additional installation requirements.

### Debugging

The Coconut tools have extensive logging built in.  In fact, we encourage liberal logging in new code, and adding new
logging when debugging problems.  This helps to ensure future debugging endeavors benefit from your sleuthing.

All logging is done using Google's [Glog library](https://github.com/golang/glog).  It is relatively bare-bones, and
adds basic leveled logging, stack dumping, and other capabilities beyond what Go's built-in logging routines offer.

The Coconut command line has two flags that control this logging and that can come in handy when debugging problems.  The
`--logtostderr` flag spews directly to stderr, rather than the default of logging to files in your temp directory.  And
the `--verbose=n` flag (`-v=n` for short) sets the logging level to `n`.  Anything greater than 3 is reserved for
debug-level logging, greater than 5 is going to be quite verbose, and anything beyond 7 is extremely noisy.

For example, the command

    $ coco eval --logtostderr -v=5

is a pretty standard starting point during debugging that will show a fairly comprehensive trace log of a compilation.

