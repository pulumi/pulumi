# MuJS

This directory contains Mu's JavaScript compiler.

It implements a subset of JavaScript with TypeScript-style type annotations, and can compile that subset into MuPack/IL.

## Building and Testing

MuJS is built independent from the overall Mu toolchain.  First clone and cd to the right place:

    $ git clone git@github.com:marapongo/mu
    $ cd mu/tools/mujs

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

