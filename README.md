<p align="center">
    <a href="https://www.pulumi.com/?utm_source=github.com&utm_medium=referral&utm_campaign=pulumi-pulumi-github-repo&utm_content=top+logo" title="Pulumi - Modern Infrastructure as Code - AWS Azure Kubernetes Containers Serverless">
        <picture>
            <source media="(prefers-color-scheme: dark)" srcset="https://www.pulumi.com/images/logo/logo-on-black.svg">
            <img src="https://www.pulumi.com/images/logo/logo-on-white.svg" alt="Pulumi" width="350">
        </picture>
    </a>
</p>

[![Slack](https://www.pulumi.com/images/docs/badges/slack.svg)](https://slack.pulumi.com/)
[![GitHub Discussions](https://img.shields.io/github/discussions/pulumi/pulumi)](https://github.com/pulumi/pulumi/discussions)
[![NPM version](https://badge.fury.io/js/%40pulumi%2Fpulumi.svg)](https://www.npmjs.com/package/@pulumi/pulumi)
[![Python version](https://badge.fury.io/py/pulumi.svg)](https://pypi.org/project/pulumi)
[![NuGet version](https://badge.fury.io/nu/pulumi.svg)](https://www.nuget.org/packages/pulumi)
[![GoDoc](https://pkg.go.dev/badge/github.com/pulumi/pulumi.svg)](https://pkg.go.dev/github.com/pulumi/pulumi)
[![License](https://img.shields.io/github/license/pulumi/pulumi)](LICENSE)

# Infrastructure as Code in any Programming Language

<a href="https://www.pulumi.com/docs/get-started/">
    <img src="https://www.pulumi.com/images/get-started.svg?" align="right" width="120">
</a>

**Pulumi Infrastructure as Code** is the easiest way to build and deploy infrastructure, of any architecture and on any cloud, using programming languages that you already know and love. Code and ship infrastructure faster with your favorite languages and tools, and embed IaC anywhere with [Automation API](https://www.pulumi.com/docs/iac/automation-api/).

Simply write code in your favorite language and Pulumi automatically provisions and manages your resources on
[AWS](https://www.pulumi.com/docs/integrations/clouds/aws/),
[Azure](https://www.pulumi.com/docs/integrations/clouds/azure/),
[Google Cloud Platform](https://www.pulumi.com/docs/integrations/clouds/gcp/),
[Kubernetes](https://www.pulumi.com/docs/integrations/clouds/kubernetes/), and [300+ providers](https://www.pulumi.com/registry/) using an
[infrastructure-as-code](https://www.pulumi.com/what-is/what-is-infrastructure-as-code/) approach.
Skip the YAML, and use standard language features like loops, functions, classes,
and package management that you already know and love.

For example, create three web servers:

```typescript
import * as aws from "@pulumi/aws";

const ami = aws.ec2.getAmiOutput({
    owners: ["amazon"],
    mostRecent: true,
    filters: [{ name: "name", values: ["al2023-ami-*-x86_64"] }],
});

for (let i = 0; i < 3; i++) {
    new aws.ec2.Instance(`web-${i}`, {
        ami: ami.id,
        instanceType: "t3.micro",
        userData: `#!/bin/bash
            echo "Hello from server ${i}" > /var/www/index.html
            nohup python3 -m http.server 80 &`,
    });
}
```

Or a simple serverless timer that archives Hacker News every day at 8:30AM:

```typescript
import * as aws from "@pulumi/aws";
import { DynamoDBClient } from "@aws-sdk/client-dynamodb";
import { DynamoDBDocumentClient, PutCommand } from "@aws-sdk/lib-dynamodb";

const snapshots = new aws.dynamodb.Table("snapshots", {
    attributes: [{ name: "id", type: "S" }],
    hashKey: "id",
    billingMode: "PAY_PER_REQUEST",
});

aws.cloudwatch.onSchedule("daily-yc-snapshot", "cron(30 8 * * ? *)", async () => {
    const res = await fetch("https://news.ycombinator.com");
    const content = await res.text();

    const ddb = DynamoDBDocumentClient.from(new DynamoDBClient({}));
    await ddb.send(new PutCommand({
        TableName: snapshots.name.get(),
        Item: { date: Date.now(), content },
    }));
});
```

Many examples are available spanning containers, serverless, and infrastructure in
[pulumi/examples](https://github.com/pulumi/examples).

Pulumi is open source under the [Apache 2.0 license](https://github.com/pulumi/pulumi/blob/master/LICENSE), supports many languages and clouds, and is easy to extend.  This
repo contains the `pulumi` CLI, language SDKs, and core Pulumi engine, and individual libraries are in their own repos.

## Welcome

<img align="right" width="400" src="/quickstart_terminal.png" alt="Pulumi quickstart terminal" />

* **[Get Started with Pulumi](https://www.pulumi.com/docs/get-started/)**: Deploy a simple application in AWS, Azure, Google Cloud, or Kubernetes using Pulumi.

* **[Learn](https://www.pulumi.com/tutorials/)**: Follow Pulumi learning pathways to learn best practices and architectural patterns through authentic examples.

* **[Examples](https://github.com/pulumi/examples)**: Browse several examples across many languages,
  clouds, and scenarios including containers, serverless, and infrastructure.

* **[Docs](https://www.pulumi.com/docs/)**: Learn about Pulumi concepts, follow user-guides, and consult the reference documentation.

* **[Registry](https://www.pulumi.com/registry/)**: Find the Pulumi Package with the resources you need. Install the package directly into your project, browse the API documentation, and start building.

* **[Secrets Management](https://www.pulumi.com/product/secrets-management/)**: Tame secrets sprawl and configuration complexity securely across all your cloud infrastructure and applications with Pulumi ESC.

* **[Community Slack](https://slack.pulumi.com/)**: Join us in Pulumi Community Slack. All conversations and questions are welcome.

* **[GitHub Discussions](https://github.com/pulumi/pulumi/discussions)**: Ask questions or share what you're building with Pulumi.

## <a name="getting-started"></a>Getting Started

[![Watch the video](/youtube_preview_image.png)](https://www.youtube.com/watch?v=Q8tw6YTD3ac)

See the [Get Started](https://www.pulumi.com/docs/get-started/) guide to quickly get started with
Pulumi on your platform and cloud of choice.

Otherwise, the following steps demonstrate how to deploy your first Pulumi program, using AWS
Serverless Lambdas, in minutes:

1. **Install**:

    To install the latest Pulumi release, run the following (see full
    [installation instructions](https://www.pulumi.com/docs/install/) for additional installation options):

    ```bash
    curl -fsSL https://get.pulumi.com/ | sh
    ```

2. **Create a Project**:

    After installing, you can get started with the `pulumi new` command:

    ```bash
    mkdir pulumi-demo && cd pulumi-demo
    pulumi new serverless-aws-typescript
    ```

    The `new` command offers [templates](https://github.com/pulumi/templates/) for all languages and clouds.  Run it without an argument and it'll prompt
    you with available projects.  This command creates an AWS Serverless Lambda project written in TypeScript.

3. **Deploy to the Cloud**:

    Run `pulumi up` to get your code to the cloud:

    ```bash
    pulumi up
    ```

    This makes all cloud resources needed to run your code.  Simply make edits to your project, and subsequent
    `pulumi up`s will compute the minimal diff to deploy your changes.

4. **Use Your Program**:

    Now that your code is deployed, you can interact with it.  In the above example, we can curl the endpoint:

    ```bash
    curl $(pulumi stack output url)
    ```

5. **Access the Logs**:

    If you're using containers or functions, Pulumi's unified logging command will show all of your logs:

    ```bash
    pulumi logs -f
    ```

6. **Destroy your Resources**:

    After you're done, you can remove all resources created by your program:

    ```bash
    pulumi destroy -y
    ```

To learn more, head over to [pulumi.com](https://www.pulumi.com/) for much more information, including
[tutorials](https://www.pulumi.com/tutorials/), [examples](https://github.com/pulumi/examples), and
details of the core Pulumi CLI and [programming model concepts](https://www.pulumi.com/docs/iac/concepts/).

## <a name="platform"></a>Platform

### Languages

|    | Language | Status | Runtime | Versions |
| -- | -------- | ------ | ------- | -------- |
| <img src="https://www.pulumi.com/logos/tech/logo-js.png" height=38 />     | [JavaScript](https://www.pulumi.com/docs/iac/languages-sdks/javascript/) | Stable  | Node.js | [Current, Active and Maintenance LTS versions](https://nodejs.org/en/about/previous-releases)  |
| <img src="https://www.pulumi.com/logos/tech/logo-ts.png" height=38 />     | [TypeScript](https://www.pulumi.com/docs/iac/languages-sdks/javascript/) | Stable  | Node.js | [Current, Active and Maintenance LTS versions](https://nodejs.org/en/about/previous-releases)  |
| <img src="https://www.pulumi.com/logos/tech/logo-python.svg" height=38 /> | [Python](https://www.pulumi.com/docs/iac/languages-sdks/python/)     | Stable  | Python | [Supported versions](https://devguide.python.org/versions/#versions) |
| <img src="https://www.pulumi.com/logos/tech/logo-golang.png" height=38 /> | [Go](https://www.pulumi.com/docs/iac/languages-sdks/go/)             | Stable  | Go | [Supported versions](https://go.dev/doc/devel/release#policy) |
| <img src="https://www.pulumi.com/logos/tech/dotnet.svg" height=38 />      | [.NET (C#/F#/VB.NET)](https://www.pulumi.com/docs/iac/languages-sdks/dotnet/)     | Stable  | .NET | [Supported versions](https://dotnet.microsoft.com/en-us/platform/support/policy/dotnet-core#lifecycle)  |
| <img src="https://www.pulumi.com/logos/tech/java.svg" height=38 />      | [Java](https://www.pulumi.com/docs/iac/languages-sdks/java/)     | Stable  | JDK | 11+  |
| <img src="https://www.pulumi.com/logos/tech/yaml.svg" height=38 />      | [YAML](https://www.pulumi.com/docs/iac/languages-sdks/yaml/)     | Stable  | n/a  | n/a  |

### Clouds

Visit the [Registry](https://www.pulumi.com/registry/) for the full list of supported cloud and infrastructure providers.

## Contributing

Visit [CONTRIBUTING.md](https://github.com/pulumi/pulumi/blob/master/CONTRIBUTING.md) for information on building Pulumi from source or contributing improvements.
