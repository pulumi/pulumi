# Pulumi Fabric Node.js SDK

The Pulumi Fabric Node.js SDK lets you write cloud programs in JavaScript.

## Installing

For now, we only support developers building from source.  Eventually we will have a nice installer.

### Prerequisites

To build and install the SDK, you will first need a few things.

First, install Node.js 6.10.2.  We recommend [nvm](https://github.com/creationix/nvm), since it makes it easier
to switch between versions of Node.js.  Afterwards, run `nvm install 6.10.2`. As part of the build, a custom build
of Node.js 6.10.2 will be downloaded and installed alongside the SDK. This Node binary is identical to Node.js 6.10.2,
except that it has one extra builtin module containing code that is used by the SDK to serialize JavaScript closures.
The custom binary is built from [this repo](https://github.com/pulumi/node).

Next, we suggest using [Yarn](https://yarnpkg.com/lang/en/docs/install/) for package management.  NPM works too, but
Yarn is faster and therefore preferred.  Please follow the directions on Yarn's website.

### Building and Testing

The first time you build, you must `make configure` to install and prepare native plugins for V8:

    $ make configure

Make sure to run this after installing the right version of Node.js above, otherwise it may bind to the wrong version.

To build the SDK, simply run `make` from the root directory (`sdk/nodejs/`).  This will build the code, run tests, and
then "install" the package (by `yarn link`ing the resulting `bin/` directory).

We recommend putting `bin/` on your `$PATH`, since the `pulumi-langhost-nodejs` executable will be loaded dynamically
by the `pulumi` tool whenever it encounters a Node.js program.

The tests will verify that everything works, but feel free to try running `pulumi preview` and/or `pulumi update` from
the `examples/minimal/` directory.  Remember to run `tsc` first, since `pulumi` expects JavaScript, not TypeScript.

