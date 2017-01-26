# Mu

Mu is a framework and toolset for creating reusable stacks of services.

If you are learning about Mu for the first time, please see [the overview document](docs/overview.md).

## Architecture

![Architecture](docs/images/arch.png)

## Building and Testing

To build Mu, first clone it into a standard Go workspace:

    $ mkdir -p $GOPATH/src/github.com/marapongo
    $ git clone git@github.com:marapongo/mu $GOPATH/src/github.com/marapongo/mu

A good default value for `GOPATH` is `~/go`.

Mu needs to know where to look for its runtime, library, etc.  By default, it will look in `/usr/local/mu`, however you
can override this with the `MUPATH` variable.  Normally it's easiest just to create a symlink:

    $ ln -s $GOPATH/src/github.com/marapongo/mu /usr/local/mu

There is one additional build-time dependency, `golint`, which can be installed using:

    $ go get -u github.com/golang/lint/golint

And placed on your path by:

    $ export PATH=$PATH:$GOPATH/bin

At this point you should be able to build and run tests from the root directory:

    $ cd $GOPATH/src/github.com/marapongo/mu
    $ make

This installs the `mu` binary into `$GOPATH/bin`, which may now be run provided `make` exited successfully.

