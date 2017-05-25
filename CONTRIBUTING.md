# Contributing to Lumi

Do you want to hack on Lumi?  Awesome!  We are so happy to have you.

This document outlines the process of contributing and the expectations and requirements of you as a contributor and
important member of the Lumi community.

## Pull Requests Welcome

First and foremost: We welcome any and all contributions from any and all contributors.  Please do not hesitate to ask
a question, make a suggestion, fix a bug, or implement a feature, no matter how big or small.

We of course would like such contributions to follow these guidelines and will most likely want the opportunity to
discuss the proposed design of major contributions before you spend considerable time creating them.  For that, reason,
we recommend engaging early and often with the core community contributors.

## Contribution Requirements

In order to ensure contributions go through smoothly, please follow these guidelines.

### Contributor License Agreements

We'd love to jump straight into the code.  Before we can do that, however, we have to jump through a couple of legal
hurdles, to protect you, us, and the Lumi project.

Namely, we need a signed individual or corporate [Contributor License Agreement (CLA)](
https://en.wikipedia.org/wiki/Contributor_License_Agreement).  If you are an individual writing original source code and
you're sure you own the intellectual property, you'll need to sign an individual CLA.  If, on the other hand, you work
for a company that wants to allow you to contribute your work, you'll need to sign a corporate CLA.

At the moment, the Lumi project doesn't have an automated system for CLA signatures.  Please ask one of the project
maintainers for the relevant document, sign and return it, and we'll keep it on file for you.

### Coding Standards

In general, our rule is to use the best in breed coding standards for the respective language.  Lumi is a multi-language
ecosystem and so the rules here differ based on which language a particular contribution is written in.  There are also
some language-agnostic rules that we follow.  Let's look at each in order.

#### Language-Specific Coding Standards

Because Lumi is multi-language, the coding standards that apply to your contribution will depend on where you are making
that contribution.  Here are the current relevant language standards.

##### Go

All Go code MUST:

* Be [`gofmt` clean](https://golang.org/cmd/gofmt/).
* Be [`golint` clean](https://github.com/golang/lint).
* Be [`go vet` clean](https://golang.org/cmd/vet/).
* Follow the [Effective Go best practices](https://golang.org/doc/effective_go.html).

##### TypeScript

All TypeScript code MUST:

* Be [`tslint` clean](https://github.com/palantir/tslint).
* Follow the [TypeScript team's coding guidelines](https://github.com/Microsoft/TypeScript/wiki/Coding-guidelines).

##### JavaScript

All JavaScript code MUST:

* Follow the [Google JavaScript style guide](https://google.github.io/styleguide/jsguide.html).

##### Python

All Python code MUST:

* Follow the [Google Python style guide](https://google.github.io/styleguide/pyguide.html).

#### Language-Agnostic Coding Standards

These are some language-agnostic rules we apply across our codebase:

* The top of each file MUST contain the standard Lumi licensing information:

    // Licensed to Pulumi Corporation ("Pulumi") under one or more
    // contributor license agreements.  See the NOTICE file distributed with
    // this work for additional information regarding copyright ownership.
    // Pulumi licenses this file to You under the Apache License, Version 2.0
    // (the "License"); you may not use this file except in compliance with
    // the License.  You may obtain a copy of the License at
    //
    //     http://www.apache.org/licenses/LICENSE-2.0
    //
    // Unless required by applicable law or agreed to in writing, software
    // distributed under the License is distributed on an "AS IS" BASIS,
    // WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    // See the License for the specific language governing permissions and
    // limitations under the License.

* There are two special kinds of comments, each of which MUST have a corresponding work item `xx`:

    - `TODO[pulumi/lumi#xx]: <comment>`: a future suggestion or improvement.
    - `BUGBUG[pulumi/lumi#xx]: <comment>`: a knowingly incorrect bit of code -- use sparingly!

* All code SHOULD use defensive coding where applicable, such as liberal assertions and contracts.

* All code SHOULD use logging extensively to facilitate future debugging endeavors.

## How to Contribute

### Developer Guide

We do not yet have a developer guide, though that's on the [TODO[pulumi/lumi#166]](
https://github.com/pulumi/lumi/issues/166) list.  For now, please refer to our [README](
https://github.com/pulumi/lumi/blob/master/README.md), as it has rudimentary instructions on enlisting, building, and
testing, plus links to the relevant tidbits throughout the repo that you might be interested in.

### Filing Issues

If you have a question about Lumi, or have a problem using it, please start with our troubleshooting guide.  (OK, OK, we
don't yet have a troubleshooting guide... someday.)  If that doesn't answer your questions, or if you think you found a
bug, please [file an issue](https://github.com/pulumi/lumi/issues/new).  We are happy to help!

### Finding Things that Need Help

If you're new to the project and want to help, but don't know where to start, we do classify certain issues as "job
jar" to indicate that they are fairly bite-sized, independent, and shouldn't require deep knowledge of the system.
[Please have a look and see if anything sounds interesting!](
https://github.com/pulumi/lumi/issues?q=is%3Aissue+is%3Aopen+label%3Astatus%2Fjob-jar).

Alternatively, you may want to peruse the [many design documents](/docs), and see if something piques your interest.
The best way to learn is to hack, so please feel free to experiment!  There is always code that can be clarified, better
documented, tested with more rigor, or improved with clearer names, all of which would be much appreciated.

### Submitting a Pull Request

If you are working on an existing work item, please be sure to let people know.  Alternatively, if you are doing work
to fix something or make an improvement for which there isn't yet a work item, please file one first, so that we have
record of the issue in advance and can comment on relevant history and/or collaborate with you on the design.

We follow the usual Git flow for contributions, so just fork the repo, develop and test your changes, and, once it is
passing all relevant tests, then submit a pull request.

Please follow these guidelines for your pull request (PR):

* PRs MUST be related to one piece of work, not a smattering of unrelated changes.
* PRs MUST have short, but descriptive, names.
* PRs MUST have informative descriptions that link to the relevant work item for the work.
* PRs MUST tag at least one desired reviewer for the change.  If you are unsure, ask in the relevant work item.
* Any and all PR feedback MUST be addressed to the satisfaction of the project maintainers.
* PRs SHOULD squash superfluous commits but retain the essential ones.

And, with that, we're done.  We sincerely look forward to seeing the amazing Lumi contributions you will make!

