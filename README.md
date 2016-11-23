# Mu

Mu is a framework and toolset for creating reusable stacks of services.

## Building and Testing

To build Mu, first clone it into a standard Go workspace:

    $ mkdir -p $GOPATH/src/github.com/marapongo
    $ git clone git@github.com:marapongo/mu $GOPATH/src/github.com/marapongo/mu

A good default value for `GOPATH` is `~/go`.

There is one additional build-time dependency, `golint`, which can be installed using:

    $ go get -u github.com/golang/lint/golint

And placed on your path by:

    $ export PATH=$PATH:$GOPATH/bin

At this point you should be able to build and run tests from the root directory:

    $ cd $GOPATH/src/github.com/marapongo/mu
    $ make

This installs the `mu` binary into `$GOPATH/bin`, which may now be run provided `make` exited successfully.

