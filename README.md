<a href="https://pulumi.io" title="Pulumi Cloud Development Platform - AWS Azure Kubernetes Containers Serverless"><img src="https://pulumi.io/images/logo/logo.svg" width="350"></a>

[![Slack](https://pulumi.io/images/badges/slack.svg)](https://slack.pulumi.io)
[![NPM version](https://badge.fury.io/js/%40pulumi%2Fpulumi.svg)](https://npmjs.com/package/@pulumi/pulumi)
[![Python version](https://badge.fury.io/py/pulumi.svg)](https://pypi.org/project/pulumi)
[![GoDoc](https://godoc.org/github.com/pulumi/pulumi?status.svg)](https://godoc.org/github.com/pulumi/pulumi)
[![License](https://img.shields.io/npm/l/%40pulumi%2Fpulumi.svg)](https://github.com/pulumi/pulumi/blob/master/LICENSE)

**The Pulumi Cloud Native Development Platform** is the easiest way to create and deploy cloud programs that use
containers, serverless functions, hosted services, and infrastructure, on any cloud.

Simply write code in your favorite language and Pulumi automatically provisions and manages your
[AWS](https://pulumi.io/reference/aws.html), [Azure](https://pulumi.io/reference/azure.html),
[Google Cloud Platform](https://pulumi.io/reference/gcp.html), and/or
[Kubernetes](https://pulumi.io/reference/kubernetes.html) resources, using an
[infrastructure-as-code](https://en.wikipedia.org/wiki/Infrastructure_as_Code) approach.  Skip the YAML, and
use standard language features like loops, functions, classes, and package management that you already know and love.

For example, create three web servers:

```typescript
let aws = require("@pulumi/aws");
let sg = new aws.ec2.SecurityGroup("web-sg", {
    ingress: [{ protocol: "tcp", fromPort: 80, toPort: 80, cidrBlocks: ["0.0.0.0/0"]}],
});
for (let i = 0; i < 3; i++) {
    new aws.ec2.Instance(`web-${i}`, {
        ami: "ami-7172b611",
        instanceType: "t2.micro",
        securityGroups: [ sg.name ],
        userData: `#!/bin/bash
            echo "Hello, World!" > index.html
            nohup python -m SimpleHTTPServer 80 &`,
    });
}
```

Or a simple serverless timer that archives Hacker News every day at 8:30AM:

```typescript
let cloud = require("@pulumi/cloud");
let snapshots = new cloud.Table("snapshots");
cloud.timer.daily("daily-yc-snapshot", { hourUTC: 8, minuteUTC: 30 }, () => {
    let req = require("https").get("https://news.ycombinator.com", (res) => {
        let content = "";
        res.setEncoding("utf8");
        res.on("data", (chunk) => { content += chunk });
        res.on("end", () => {
           snapshots.insert({ date: Date.now(), content: content });
        });
    });
    req.end();
});
```

Many examples are available spanning containers, serverless, and infrastructure in
[pulumi/examples](https://github.com/pulumi/examples).

Pulumi is open source under the Apache 2.0 license, supports many languages and clouds, and is easy to extend.  This
repo contains the `pulumi` CLI, language SDKs, and core Pulumi engine, and individual libraries are in their own repos.

## Welcome

<img align="right" width="400" src="https://pulumi.io/images/quickstart/console.png" />

* **[Getting Started](#getting-started)**: get up and running quickly.

* **[Tutorials](https://pulumi.io/quickstart)**: walk through end-to-end workflows for creating containers, serverless
  functions, and other cloud services and infrastructure.

* **[Examples](https://github.com/pulumi/examples)**: browse a number of useful examples across many languages,
  clouds, and scenarios including containers, serverless, and infrastructure.

* **[A Tour of Pulumi](https://pulumi.io/tour)**: interactively walk through the core Pulumi concepts, one at a time,
  covering the entire CLI and programming model surface area in a handful of bite-sized chunks.

* **[Reference Docs](https://pulumi.io/reference)**: read conceptual documentation, in addition to details on how
  to configure Pulumi to deploy into your AWS, Azure, or Google Cloud accounts, and/or Kubernetes cluster.

* **[Community Slack](https://slack.pulumi.io)**: join us over at our community Slack channel.  Any and all
  discussion or questions are welcome.

## <a name="getting-started"></a>Getting Started

Follow these steps to deploy your first Pulumi program, using AWS Serverless Lambdas, in minutes:

1. **Install**:

    To install the latest Pulumi release, run:

    ```bash
    $ curl -fsSL https://get.pulumi.com/ | sh
    ```

2. **[Configure your Cloud Provider](https://pulumi.io/install#cloud-configuration)** so that Pulumi can deploy into it.

3. **Create a Project**:

    After installing, you can get started with the `pulumi new` command:

    ```bash
    $ pulumi new hello-aws-javascript
    ```

    The `new` command offers templates for all languages and clouds.  Run it without an argument and it'll prompt
    you with available projects.  This command created an AWS Serverless Lambda project written in JavaScript.

4. **Deploy to the Cloud**:

    Run `pulumi update` to get your code to the cloud:

    ```bash
    $ pulumi update
    ```

    This makes all cloud resources needed to run your code.  Simply make edits to your project, and subsequent
    `pulumi update`s will compute the minimal diff to deploy your changes.

5. **Use Your Program**:

    Now that your code is deployed, you can interact with it.  In the above example, we can curl the endpoint:

    ```bash
    $ curl $(pulumi stack output url)
    ```

6. **Access the Logs**:

    If you're using containers or functions, Pulumi's unified logging command will show all of your logs:

    ```bash
    $ pulumi logs -f
    ```

7. **Destroy your Resources**:

    After you're done, you can remove all resources created by your program:

    ```bash
    $ pulumi destroy -y
    ```

Please head on over to [the project website](https://pulumi.io) for much more information, including
[tutorials](https://pulumi.io/quickstart), [examples](https://github.com/pulumi/examples), and
[an interactive tour](https://pulumi.io/tour) of the core Pulumi CLI and programming model concepts.

## <a name="platform"></a>Platform

### CLI

| Architecture | Build Status |
| ------------ | ------------ |
| Linux/macOS x64 | [![Linux x64 Build Status](https://travis-ci.com/pulumi/pulumi.svg?token=cTUUEgrxaTEGyecqJpDn&branch=master)](https://travis-ci.com/pulumi/pulumi) |
| Windows x64  | [![Windows x64 Build Status](https://ci.appveyor.com/api/projects/status/uqrduw6qnoss7g4i?svg=true&branch=master)](https://ci.appveyor.com/project/pulumi/pulumi) |

### Languages

|    | Language | Status | Runtime |
| -- | -------- | ------ | ------- |
| <img src="https://www.pulumi.com/assets/logos/tech/logo-js.png" height=38 /> | [JavaScript](./sdk/nodejs) | Stable | Node.js 6.x-10.x |
| <img src="https://www.pulumi.com/assets/logos/tech/logo-ts.png" height=38 /> | [TypeScript](./sdk/nodejs) | Stable | Node.js 6.x-10.x |
| <img src="https://www.pulumi.com/assets/logos/tech/logo-python.png" height=38 /> | [Python](./sdk/python) | Preview | Python 2.7 |
| <img src="https://www.pulumi.com/assets/logos/tech/logo-golang.png" height=38 /> | [Go](./sdk/go) | Preview | Go 1.x |

### Clouds

|    | Cloud | Status | Docs |
| -- | ----- | ------ | ---- |
| <img src="https://www.pulumi.com/assets/logos/tech/logo-aws.png" height=38 /> | [Amazon Web Services](https://github.com/pulumi/pulumi-aws) | Stable | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/aws/) |
| <img src="https://www.pulumi.com/assets/logos/tech/logo-azure.png" height=38 /> | [Microsoft Azure](https://github.com/pulumi/pulumi-azure) | Preview | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/azure/) |
| <img src="https://www.pulumi.com/assets/logos/tech/logo-gd.png" height=38 /> | [Google Cloud Platform](https://github.com/pulumi/pulumi-gcp) | Preview | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/gcp/) |
| <img src="https://www.pulumi.com/assets/logos/tech/logo-kubernetes.png" height=38 /> | [Kubernetes](https://github.com/pulumi/pulumi-kubernetes) | Preview | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/kubernetes/) |

### Libraries

There are several libraries that encapsulate best practices and common patterns:

| Library | Status | Docs | Repo |
| ------- | ------ | ---- | ---- |
| AWS Serverless | Preview | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/aws-serverless/) | [pulumi/pulumi-aws-serverless](https://github.com/pulumi/pulumi-aws-serverless) |
| AWS Infrastructure | Preview | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/aws-infra/) | [pulumi/pulumi-aws-infra](https://github.com/pulumi/pulumi-aws-infra) |
| Pulumi Multi-Cloud Framework | Preview | [Docs](https://pulumi.io/reference/pkg/nodejs/@pulumi/cloud/) | [pulumi/pulumi-cloud](https://github.com/pulumi/pulumi-cloud) |

## Development

If you'd like to contribute to Pulumi and/or build from source, this section is for you.

### Prerequisites

Pulumi is written in Go, uses Dep for dependency management, and GoMetaLinter for linting:

* [Go](https://golang.org/doc/install): https://golang.org/dl
* [Dep](https://github.com/golang/dep): `$ go get -u github.com/golang/dep/cmd/dep`
* [GoMetaLinter](https://github.com/alecthomas/gometalinter):
    - `$ go get -u github.com/alecthomas/gometalinter`
    - `$ gometalinter --install`

### Building and Testing

To install the pre-built SDK, please run `curl -fsSL https://get.pulumi.com/ | sh`, or see detailed installation instructions on [the project page](https://pulumi.io/).  Read on if you want to install from source.

To build a complete Pulumi SDK, ensure `$GOPATH` is set, and clone into a standard Go workspace:

    $ git clone git@github.com:pulumi/pulumi $GOPATH/src/github.com/pulumi/pulumi
    $ cd $GOPATH/src/github.com/pulumi/pulumi

The first time you build, you must `make ensure` to install dependencies and perform other machine setup:

    $ make ensure

In the future, you can synch dependencies simply by running `dep ensure` explicitly:

    $ dep ensure

At this point you can run `make` to build and run tests:

    $ make

This installs the `pulumi` binary into `$GOPATH/bin`, which may now be run provided `make` exited successfully.

The Makefile also supports just running tests (`make test_all` or `make test_fast`), just running the linter
(`make lint`), just running Govet (`make vet`), and so on.  Please just refer to the Makefile for the full list of targets.

### Debugging

The Pulumi tools have extensive logging built in.  In fact, we encourage liberal logging in new code, and adding new
logging when debugging problems.  This helps to ensure future debugging endeavors benefit from your sleuthing.

All logging is done using Google's [Glog library](https://github.com/golang/glog).  It is relatively bare-bones, and
adds basic leveled logging, stack dumping, and other capabilities beyond what Go's built-in logging routines offer.

The `pulumi` command line has two flags that control this logging and that can come in handy when debugging problems.
The `--logtostderr` flag spews directly to stderr, rather than the default of logging to files in your temp directory.
And the `--verbose=n` flag (`-v=n` for short) sets the logging level to `n`.  Anything greater than 3 is reserved for
debug-level logging, greater than 5 is going to be quite verbose, and anything beyond 7 is extremely noisy.

For example, the command

    $ pulumi preview --logtostderr -v=5

is a pretty standard starting point during debugging that will show a fairly comprehensive trace log of a compilation.
