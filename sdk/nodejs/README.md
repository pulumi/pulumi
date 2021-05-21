# Pulumi Node.js SDK

The Pulumi Node.js SDK lets you write cloud programs in JavaScript.

## Installation

Using npm:

```bash
$ npm install --save @pulumi/pulumi
```

Using yarn:

```bash
$ yarn add @pulumi/pulumi
```

This SDK is meant for use with the Pulumi CLI.  Please visit
[pulumi.com](https://pulumi.com) for installation instructions.

## Building and Testing

For anybody who wants to build from source, here is how you do it.

### Prerequisites

This SDK uses Node.js and we support any of the
[Current, Active and Maintenance LTS versions](https://nodejs.org/en/about/releases/). We support both
[NPM](https://npmjs.org) and [Yarn](https://yarnpkg.com/lang/en/docs/install/) for package management.

At the moment, we only support building on macOS and Linux, where standard GNU tools like `make` are available.

### Make Targets

To build the SDK, simply run `make` from the root directory (where this `README` lives, at `sdk/nodejs/` from the repo's
root).  This will build the code, run tests, and install the package and its supporting artifacts.

At the moment, for local development, we install everything into `/opt/pulumi`.  You will want this on your `$PATH`.

The tests will verify that everything works, but feel free to try running `pulumi preview` and/or `pulumi up` from
the `examples/minimal/` directory.  Remember to run `tsc` first, since `pulumi` expects JavaScript, not TypeScript.
