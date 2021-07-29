<a href="https://www.pulumi.com?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=top-logo" title="Pulumi - Modern Infrastructure as Code - AWS Azure Kubernetes Containers Serverless">
    <img src="https://www.pulumi.com/images/logo/logo-on-white-box.svg?" width="350">
</a>

[![Slack](http://www.pulumi.com/images/docs/badges/slack.svg)](https://slack.pulumi.com?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=slack-badge)
![GitHub Discussions](https://img.shields.io/github/discussions/pulumi/pulumi)
[![NPM version](https://badge.fury.io/js/%40pulumi%2Fpulumi.svg)](https://npmjs.com/package/@pulumi/pulumi)
[![Python version](https://badge.fury.io/py/pulumi.svg)](https://pypi.org/project/pulumi)
[![NuGet version](https://badge.fury.io/nu/pulumi.svg)](https://badge.fury.io/nu/pulumi)
[![GoDoc](https://godoc.org/github.com/pulumi/pulumi?status.svg)](https://godoc.org/github.com/pulumi/pulumi)
[![License](https://img.shields.io/npm/l/%40pulumi%2Fpulumi.svg)](https://github.com/pulumi/pulumi/blob/master/LICENSE)
[![Gitpod ready-to-code](https://img.shields.io/badge/Gitpod-ready--to--code-blue?logo=gitpod)](https://gitpod.io/#https://github.com/pulumi/pulumi)

<a href="https://www.pulumi.com/docs/get-started/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=get-started-button" title="Get Started">
    <img src="https://www.pulumi.com/images/get-started.svg?" align="right" width="120">
</a>

**Pulumi's Infrastructure as Code SDK** is the easiest way to create and deploy cloud software that use
containers, serverless functions, hosted services, and infrastructure, on any cloud.

Simply write code in your favorite language and Pulumi automatically provisions and manages your
[AWS](https://www.pulumi.com/docs/reference/clouds/aws/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=aws-reference-link),
[Azure](https://www.pulumi.com/docs/reference/clouds/azure/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=azure-reference-link),
[Google Cloud Platform](https://www.pulumi.com/docs/reference/clouds/gcp/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=gcp-reference-link), and/or
[Kubernetes](https://www.pulumi.com/docs/reference/clouds/kubernetes/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=kuberneters-reference-link) resources, using an
[infrastructure-as-code](https://en.wikipedia.org/wiki/Infrastructure_as_Code) approach.
Skip the YAML, and use standard language features like loops, functions, classes,
and package management that you already know and love.

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
const aws = require("@pulumi/aws");

const snapshots = new aws.dynamodb.Table("snapshots", {
    attributes: [{ name: "id", type: "S", }],
    hashKey: "id", billingMode: "PAY_PER_REQUEST",
});

aws.cloudwatch.onSchedule("daily-yc-snapshot", "cron(30 8 * * ? *)", () => {
    require("https").get("https://news.ycombinator.com", res => {
        let content = "";
        res.setEncoding("utf8");
        res.on("data", chunk => content += chunk);
        res.on("end", () => new aws.sdk.DynamoDB.DocumentClient().put({
            TableName: snapshots.name.get(),
            Item: { date: Date.now(), content },
        }).promise());
    }).end();
});
```

Many examples are available spanning containers, serverless, and infrastructure in
[pulumi/examples](https://github.com/pulumi/examples).

Pulumi is open source under the Apache 2.0 license, supports many languages and clouds, and is easy to extend.  This
repo contains the `pulumi` CLI, language SDKs, and core Pulumi engine, and individual libraries are in their own repos.

## Welcome

<img align="right" width="400" src="https://www.pulumi.com/images/docs/quickstart/console.png" />

* **[Getting Started](#getting-started)**: get up and running quickly.

* **[Tutorials](https://www.pulumi.com/docs/reference/tutorials/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=welcome-tutorials)**: walk through end-to-end workflows for creating containers, serverless
  functions, and other cloud services and infrastructure.

* **[Examples](https://github.com/pulumi/examples)**: browse a number of useful examples across many languages,
  clouds, and scenarios including containers, serverless, and infrastructure.

* **[Reference Docs](https://www.pulumi.com/docs/reference/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=welcome-reference-docs)**: read conceptual documentation, in addition to details on how
  to configure Pulumi to deploy into your AWS, Azure, or Google Cloud accounts, and/or Kubernetes cluster.

* **[Community Slack](https://slack.pulumi.com/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=welcome-slack)**: join us over at our community Slack channel.  Any and all
  discussion or questions are welcome.
* **[GitHub Discussions](https://github.com/pulumi/pulumi/discussions)**: Ask your questions or share what you're building with Pulumi.

* **[Roadmap](https://github.com/pulumi/pulumi/wiki/Roadmap)**: check out what's on the roadmap for the Pulumi
  project over the coming months.

## <a name="getting-started"></a>Getting Started

[![Watch the video](/youtube_preview_image.png)](https://www.youtube.com/watch?v=6f8KF6UGN7g)

See the [Get Started](https://www.pulumi.com/docs/quickstart/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=getting-started-quickstart) guide to quickly get started with
Pulumi on your platform and cloud of choice.

Otherwise, the following steps demonstrate how to deploy your first Pulumi program, using AWS
Serverless Lambdas, in minutes:

1. **Install**:

    To install the latest Pulumi release, run the following (see full
    [installation instructions](https://www.pulumi.com/docs/reference/install/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=getting-started-install) for additional installation options):

    ```bash
    $ curl -fsSL https://get.pulumi.com/ | sh
    ```

2. **Create a Project**:

    After installing, you can get started with the `pulumi new` command:

    ```bash
    $ mkdir pulumi-demo && cd pulumi-demo
    $ pulumi new hello-aws-javascript
    ```

    The `new` command offers templates for all languages and clouds.  Run it without an argument and it'll prompt
    you with available projects.  This command created an AWS Serverless Lambda project written in JavaScript.

3. **Deploy to the Cloud**:

    Run `pulumi up` to get your code to the cloud:

    ```bash
    $ pulumi up
    ```

    This makes all cloud resources needed to run your code.  Simply make edits to your project, and subsequent
    `pulumi up`s will compute the minimal diff to deploy your changes.

4. **Use Your Program**:

    Now that your code is deployed, you can interact with it.  In the above example, we can curl the endpoint:

    ```bash
    $ curl $(pulumi stack output url)
    ```

5. **Access the Logs**:

    If you're using containers or functions, Pulumi's unified logging command will show all of your logs:

    ```bash
    $ pulumi logs -f
    ```

6. **Destroy your Resources**:

    After you're done, you can remove all resources created by your program:

    ```bash
    $ pulumi destroy -y
    ```

To learn more, head over to [pulumi.com](https://pulumi.com/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=getting-started-learn-more-home) for much more information, including
[tutorials](https://www.pulumi.com/docs/reference/tutorials/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=getting-started-learn-more-tutorials), [examples](https://github.com/pulumi/examples), and
details of the core Pulumi CLI and [programming model concepts](https://www.pulumi.com/docs/reference/concepts/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=getting-started-learn-more-concepts).

## <a name="platform"></a>Platform

### CLI

| Architecture | Build Status |
| ------------ | ------------ |
| Linux/macOS x64 | [![Linux x64 Build Status](https://travis-ci.com/pulumi/pulumi.svg?token=cTUUEgrxaTEGyecqJpDn&branch=master)](https://travis-ci.com/pulumi/pulumi)                |
| Windows x64     | [![Windows x64 Build Status](https://ci.appveyor.com/api/projects/status/uqrduw6qnoss7g4i?svg=true&branch=master)](https://ci.appveyor.com/project/pulumi/pulumi) |

### Languages

|    | Language | Status | Runtime |
| -- | -------- | ------ | ------- |
| <img src="https://www.pulumi.com/logos/tech/logo-js.png" height=38 />     | [JavaScript](./sdk/nodejs) | Stable  | Node.js 12+  |
| <img src="https://www.pulumi.com/logos/tech/logo-ts.png" height=38 />     | [TypeScript](./sdk/nodejs) | Stable  | Node.js 12+  |
| <img src="https://www.pulumi.com/logos/tech/logo-python.png" height=38 /> | [Python](./sdk/python)     | Stable  | Python 3.6+ |
| <img src="https://www.pulumi.com/logos/tech/logo-golang.png" height=38 /> | [Go](./sdk/go)             | Stable  | Go 1.14+   |
| <img src="https://www.pulumi.com/logos/tech/dotnet.png" height=38 />      | [.NET (C#/F#/VB.NET)](./sdk/dotnet)     | Stable  | .NET Core 3.1+  |

### Clouds

See [Supported Clouds](https://www.pulumi.com/docs/reference/clouds/?utm_campaign=pulumi-pulumi-github-repo&utm_source=github.com&utm_medium=clouds) for the
full list of supported cloud and infrastructure providers.

## Contributing

Please See [CONTRIBUTING.md](https://github.com/pulumi/pulumi/blob/master/CONTRIBUTING.md)
for information on building Pulumi from source or contributing improvements.
