# CocoJS

This directory contains Coconut's JavaScript compiler.

It implements a subset of JavaScript, with optional TypeScript-style type annotations, and compiles that subset into
CocoPack/IL.

## Building and Testing

CocoJS is built independent from the overall Coconut toolchain.  First clone and cd to the right place:

    $ git clone git@github.com:pulumi/coconut
    $ cd coconut/tools/cocojs

Next, install dependencies, ideally [using Yarn](https://yarnpkg.com/en/docs/install):

    $ yarn install

(NPM can be used instead, but Yarn offers better performance, reliability, and security, so it's what we use below.)

From there, to build:

    $ yarn run build

It's possible to simply run the TypeScript compiler using `tsc`, however the Yarn build step performs a couple extra
steps; namely, it runs TSLint and also copies some test baseline files into the right place.

Next, to test, simply run:

    $ yarn run test

It will be obvious if the tests passed or failed and, afterwards, code coverage data will be output to the console.

## Libraries

In order to use the Coconut libraries -- including the standard library -- you will need to do a few additional steps
to prepare your developer workspace.  Please see [this document](/lib/README.md) for details on how to do this.

