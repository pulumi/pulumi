# coconut/lib

This directory contains some libraries that Coconut programs may depend upon.

## Overview

The Coconut standard library underneath `coconut/` is special in that every program will ultimately use it directly or
indirectly to create resources.

Similarly, `cocojs/` is the CocoJS compiler's runtime library, and so most CocoJS programs will on it.

Note that these are written in the Coconut subsets of the languages and therefore cannot perform I/O, etc.

## Installation and Usage

Eventually these packages will be published like any other NPM package.  For now, they are consumed only in a
development capacity, and so there are some manual steps required to prepare a developer workspace.

For each library `<lib>` you wish to use, please see its `install.sh` script in its root directory.  This performs
installation so that it can be used simply by adding a dependency to it.

We currently use NPM/Yarn symlinks to ease the developer workspace flow.  As such, you will need to run:

* `yarn link <lib>`

In a project that intends to consume `<lib>` before actually using it.  For example, `yarn link @coconut/coconut`.

