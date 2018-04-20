# Pulumi Fabric Node.js SDK

The Pulumi Fabric Node.js SDK lets you write cloud programs in JavaScript.

## Installing

For now, we only support developers building from source.  Eventually we will have a nice installer.

### Prerequisites

To build and install the SDK, you will first need a few things.

First, you will need a version of Node. We officially support the current node Active LTS releases and
the most recent Current release, as defined by [this table](https://github.com/nodejs/Release#release-schedule).

Next, we suggest using [Yarn](https://yarnpkg.com/lang/en/docs/install/) for package management.  NPM works too, but
Yarn is faster and therefore preferred.  Please follow the directions on Yarn's website.

### Building and Testing

The first time you build, you can `make ensure` to install and prepare native plugins for V8:

    $ make configure

This is only necessary if you intend to produce a build that is capable of running older versions of the SDK
contained in this directory. If you do intend to do this, you must have node `6.10.2` installed.

To build the SDK, simply run `make` from the root directory (`sdk/nodejs/`).  This will build the code, run tests, and
then "install" the package (by `yarn link`ing the resulting `bin/` directory).

We recommend putting `bin/` on your `$PATH`, since the `pulumi-langhost-nodejs` executable will be loaded dynamically
by the `pulumi` tool whenever it encounters a Node.js program.

The tests will verify that everything works, but feel free to try running `pulumi preview` and/or `pulumi update` from
the `examples/minimal/` directory.  Remember to run `tsc` first, since `pulumi` expects JavaScript, not TypeScript.

