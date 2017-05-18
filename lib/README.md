# lumi/lib

This directory contains some libraries that Lumi programs may depend upon.

## Overview

The Lumi standard library underneath `lumi/` is special in that every program will ultimately use it directly or
indirectly to create resources.

Similarly, `lumijs/` is the LumiJS compiler's runtime library, and so most LumiJS programs will on it.

Note that these are written in the Lumi subsets of the languages and therefore cannot perform I/O, etc.

## Installation and Usage

Eventually these packages will be published like any other NPM package.  For now, they are consumed only in a
development capacity, and so there are some manual steps required to prepare a developer workspace.

For each library `<lib>` you wish to use, please see its `install.sh` script in its root directory.  This performs
installation so that it can be used simply by adding a dependency to it.

We currently use NPM/Yarn symlinks to ease the developer workspace flow.  As such, you will need to run:

* `yarn link <lib>`

In a project that intends to consume `<lib>` before actually using it.  For example, `yarn link @lumi/lumi`.

