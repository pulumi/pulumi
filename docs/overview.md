# Coconut Overview

Coconut is a toolset and runtime for creating reusable cloud services.  The resulting packages can be shared and
consumed similar to your favorite programming language's package manager.  Coconut is inherently multi-language and
multi-cloud, and lets you build abstractions that span different cloud environments and topologies, if you desire.

This document provides an overview of the Coconut system, its goals, primary concepts, and the system architecture.

## Problem

Cloud services are difficult to build, deploy, and manage.  The current trend to use increasingly fine-grained
microservices increases complexity, transforming most modern cloud applications into complex distributed systems,
without much of the supporting language, library, and tooling support you'd expect in a distributed programming model.

There are many aspects to building distributed systems that aren't evident to the newcomer: configuration, RPC,
dependency management, logging, fault tolerance, and zero-downtime deployments, to name a few.  Even the experienced
practitioner will quickly find that today's developer and operations tools do not guide us down the golden path.  It's
common to need to master a dozen tools before even getting an application up and running in production.  Worse, these
tools are closer to ad-hoc scripts, markup files, and unfamiliar templating systems, than real programming languages.

On top of that complexity, it is difficult to share knowledge.  Most modern programming languages have component models
that allow you to encapsulate complex functionality underneath simple, easy-to-use abstractions, and package managers
that allow you to share these components with others, and consume components shared by others, in the form of libraries.
The current way that cloud architectures are built and deployed has no such componentization, sharing, or reuse.
Instead, "sharing" happens through copy-and-paste, resulting in all the standard manageability woes you would expect.

The cloud platforms are also divergent in the configuration formats that they accept, infrastructure abstractions that
they provide, and the specific knobs used to configure those abstractions.  It is as though every web application
developer needs to use a different programming language, while understanding the intricate details of the underlying
thread scheduler, filesystem, and networking stack, just to target Linux, macOS, or Windows.  This is not how things are
done: instead, most programmers use a language like Node.js, Python, Go, Java, or Ruby, to hide these details.

Finally, once such an application is up and running, managing and evolving it requires similarly ad-hoc and
individualized tools and practices.  Applying changes to a running environment is often done manually, in an unauditable
way, and patches are applied unevenly and inconsistently, incurring security and reliability hazards.

All of the above is a big productivity drain, negatively impacting the agility of organizations moving to and innovating
in the cloud.  Containers have delivered a great improvement to productivity and management of single nodes in a
cluster, but have yet to simplify entire applications or entire clusters.  By adopting concepts and ideas that have
been proven in the overall landscape of languages and runtimes, however, Coconut significantly improves this situation.

## Solution

Coconut lets developers author cloud components in their language of choice (JavaScript, Python, Ruby, Go, etc).  These
components include infrastructure, service and application-level components, serverless, and entire fabrics of
topologies.  Coconut embraces an [immutable infrastructure](http://chadfowler.com/2013/06/23/immutable-deployments.html)
philosophy, improving analysis, automation, auditing, and repeatability.  In [cattle versus pets](
https://blog.engineyard.com/2014/pets-vs-cattle) terminology, Coconut prefers cattle, but can cater to pets.

Coconut's approach to using a real language and package manager is in contrast to most approaches to cloud configuration
today which typically use obscure configuration markups, DSLs, and, occasionally, templating functionality for
conditional logic.  This unified view is particularly helpful when building serverless applications where you want to
focus on code.  In such applications, the infrastructure management pieces increasingly fade into the background.

At the same time, Coconut is polyglot, allowing composition of components authored in many different languages.  Cloud
components can be built by reusing existing components shared by others, and published to the Coconut package manager
for others to use.  A common intermediate format and runtime is used to stitch these components together.  Cloud
services are in fact simply instances of these components, with property values configured appropriately.  Change
management leverages existing source control workflows and is performed by Coconut's understanding of the overall graph
of dependencies between those services.  Think of each service as an "object" that is running in the cloud.

Coconut may target any public or private cloud, and may be used in an unopinionated way (programmatic IaaS), or fully
opinionated (more PaaS, FaaS, or BaaS-like), depending on your preference.  Although you are free to program directly to
your cloud provider's specific abstractions, using the full power of your native cloud, Coconut also facilitates
building higher-level cloud-neutral components that can run anywhere.  This includes compute services, storage services,
and even more logical domain-specific services like AI, ML, and recognition.  This is done using the standard techniques
all programmers are familiar with: interface-based abstraction, class-based encapsulation, sharing, and reuse.

## Example

Let us look at two brief examples in Coconut's flavor of JavaScript, CocoJS.

### Basic Infrastructure

The first example is an unopinionated description of infrastructure that projects onto AWS in a straightforward way.
It creates a virtual private cloud (VPC) and a subnet per availability zone, the foundation for most new private clouds:

    import * as aws from "@coconut/aws";
    
    export let vpcCidrBlock = "10.0.0.0/16";
    
    let vpc = new aws.ec2.VPC("acmecorp-cloud", {
        cidrBlock: vpcCidrBlock,
    });
    let subnets = [];
    for (let zone of aws.util.availabilityZones()) {
        subnets.push(new aws.ec2.Subnet("acmecorp-subnet-" + zone, {
            vpc: vpc,
            availabilityZone: zone,
            cidrBlock: aws.util.suggestSubnetCidr(vpcCidrBlock, zone),
        });
    }

Although this is a simple example, it shows off some of the underlying power of the Coconut model:

* Full configuration of AWS resources is available to us.
* Resources are created and configured using standard language constructors and parameters.
* The full power of a language is available to us: classes, loops, branches, function calls, etc.
* The AWS resources themselves, in fact, are just classes exported from an imported library.
* Passing resource arguments -- like `vpc` to `Subnet` -- is capability-based instead of weak string IDs.
* The `vpcCidrBlock` export is configurable and can be changed if the default `10.0.0.0/16` isn't right.
* This library can also export helpful functions, like `aws.util.availabilityZones`, avoiding hard-coding.

This alone is a significant step forward compared to what IT and developers are accustomed to.

### A Little Abstraction

The above example demonstrates an executable package that creates a VPC and its subnets.  It is a small step to package
up this code into a reusable library that others can use.

Packaging up abstractions is useful for many reasons.  Perhaps it just saves you from typing the same boilerplate
resource configuration logic over and over again, as is common in today's infrastructure markup.  Or maybe you just
want to share common patterns within your organization, to enforce some degree of consistency.  Finally, perhaps you
wish to share your creation with the overall community, so that others can leverage your hard work.

No matter the reason, doing this is as simple as exporting your infrastructure logic as a class that others can use:

    import * as aws from "@coconut/aws";

    export class AWSInfraFoundation {
        public readonly vpc: aws.ec2.VPC;
        public readonly subnets: aws.ec2.Subnet[];

        constructor(name: string, args?: AWSInfraFoundationArgs) {
            this.vpc = new aws.ec2.VPC(name + "-cloud", {
                cidrBlock: args.vpcCidrBlock || "10.0.0.0/16",
            });
            this.subnets = [];
            for (let zone of aws.util.availabilityZones()) {
                this.subnets.push(new aws.ec2.Subnet("acmecorp-subnet-" + zone, {
                    vpc: this.vpc,
                    availabilityZone: zone,
                    cidrBlock: aws.util.suggestSubnetCidr(vpcCidrBlock, zone),
                });
            }
        }
    }

    export interface AWSInfraFoundationArgs {
        vpcCidrBlock?: string; // an optional CIDR block for the VPC (default: 10.0.0.0/16).
    }

Now, given this definition exported from a library `acmecorp/infra`, another executable package can instantiate it:

    import {AWSInfraFoundation} from "...";
    let infra = new AWSInfraFoundation("acmecorp");

This small program essentially has the same effect as the original example.  And the VPC and Subnet resource objects are
trivially accessible, and useable in the same ways, simply by accessing the properties `infra.vpc` and `infra.subnets`.

### A Little Syntactic Sugar

At the bottom of the overall ecosystem of libraries are the basic resource libraries for cloud resources like AWS,
Azure, Google Cloud, Kubernetes, VMWare, and so on.  These resources project the core capabilities of these low-level
resources in their full glory, typically with very little on top (except for some types and schema for convenience).

As we will soon see in the section immediately following this one, at the top of this ecosystem are cloud-neutral,
high-level abstractions.

In the middle, however, there are some abstractions that still deliver the full power of the underlying abstractions,
but add a modest amount of convenience.  The most notable example is in the serverless domain, where cloud functions --
such as AWS Lambda -- can be expressed using ordinary lambdas.

Let's take an example.

To create an AWS Lambda using the lowest level, raw projection of the resources, we either need to express the code
using inline code embedded in a string, or we need to upload that code to S3 and reference it by bucket name.  Either
way, the expression of this is unnatural to say the least.  For example:

    import * as aws from "@coconut/aws";

    let dst = new aws.s3.Bucket("thumbnails");

    let thumbnailer = new aws.lambda.Lambda("thumbnailer", {
        handler: "index.handler",
        runtime: "nodejs",
        code: {
            zipFile:
                "var config = require('config');\n" +
                "var request = require('request');\n" +
                "var AWS = require('aws-sdk');\n" +
                "var S3 = new AWS.S3();\n" +
                "exports.handler = function(event, context) {\n" +
                "   var src = event.Records[0].s3.bucket.name;\n" +
                "   var key = event.Records[0].s3.object.key;\n" +
                "   s3.getObject(\n" +
                "       {\n" +
                "           Bucket: srcBucket,\n" +
                "           Key: srcKey,\n" +
                "       },\n" +
                "       (data, err) => {\n" +
                "           if (err) {\n" +
                "               console.error(`Unable to resize: ${err}`);\n" +
                "               context.done();\n" +
                "           }\n" +
                "           else {\n" +
                "               let thumb = generateThumbnail(data);\n" +
                "               let dstBuck = config.buckets[" + dst.id + "];\n" +
                "               request.post(dstBuck.host + '/' + key,\n" +
                "                   {\n" +
                "                       form: {\n" +
                "                           bucket: dstBuck.bucket,\n" +
                "                           secret: dstBuck.secret,\n" +
                "                       }\n" +
                "                   },\n" +
                "                   (err, response) {\n" +
                "                       if (err) {\n" +
                "                           console.error(`Unable to post thumbnail: ${err});\n" +
                "                       }\n" +
                "                       context.done();\n" +
                "                   }\n" +
                "               );\n" +
                "           }\n" +
                "       }\n" +
                "   );\n" +
                "};\n",
        },
        memory: 1024,
    });

    let src = new aws.s3.Bucket("images", {
        notificationConfiguration: {
            lambdaConfigurations: [{
                event: "s3:ObjectCreated:*",
                function: thumbnailer,
            }],
        },
    });

Although a direct translation of the underlying AWS APIs -- and remarkably similar to the AWS CloudFormation expression
of such a topology -- this example suffers from many obvious problems.  Embedding source as text leads to a poor tooling
experience overall.  The subscription to the S3 bucket event source is awkward.  And wiring up the destination bucket
requires that we dynamically inject code as text into the lambda, and fetch configuration through awkward API calls that
make the inner code feel completely and utterly disconnected from the outer code, which of course, it actually is.

The Coconut high-level abstractions alleviate this problem, as we will see soon.  But even without eschewing the
AWS-specific nature of lambdas and S3 subscriptions, we can use the `aws.x` package for considerable convenience:

    import * as aws from "@coconut/aws";

    let src = new aws.s3.Bucket("images");
    let dst = new aws.s3.Bucket("thumbnails", {
        onNewObject: new aws.x.lambda.Lambda("thumbnailer", {
            code: async (event, context) => {
                // Generate a thumbnail from the object payload.
                let thumb = generateThumbnail(event.getObject().data);
                // Now store the new thumbnail and raise our event.
                await dst.putObject(event.key, thumb);
            },
            memory: 1024,
        })
    });

Notice how concise this code is in comparison.  This is for numerous reasons.  First, we can use a real lambda in the
source language to represent the lambda's code.  Next, the lambda can simply close over variables like `src` and `dst`,
as capabilities, rather than awkwardly marshaling IDs as strings statically at compile-time.  Next, we have first class
APIs on the various context objects, so that we can simply call `getObject` and `putObject` on those capabilities,
versus marshaling everything in a very dynamically typed way.  And yet, notice we can still set the underlying
properties that we care about, like the memory size.  We have not lost any power with this gain in expressiveness.

### Advanced High-Level Abstractions

This next example demonstrates how higher-level opinionated abstractions may be built out of these fundamental
building blocks.  Note that there is nothing "special" per-se about these higher level abstractions.  These capabilities
fall out as a natural consequence from our choice of using a real programming language and real component model.

Let us now look at a cloud-neutral serverless component that creates thumbnails from images uploaded to a bucket:

    import * as coconut from "@coconut/coconut";

    export class Thumbnailer {
        // onNewThumbnail is an event that is raised for each new thumbnail.
        public readonly onNewThumbnail: coconut.x.Event;

        private readonly src: coconut.x.Bucket; // the source to monitor for images.
        private readonly coconut.x.Bucket; // the destination to store thumbnails in.

        constructor(src: coconut.x.Bucket, dst: coconut.x.Bucket) {
            this.src = src;
            this.dst = dst;
            this.onNewThumbnail = new coconut.x.Event();
            this.src.onNewObject(async (event: coconut.x.NewObjectEvent) => {
                // Generate a thumbnail from the object payload.
                let thumb = generateThumbnail(event.getObject().data);
                // Now store the new thumbnail and raise our event.
                await this.dst.putObject(event.key, thumb);
                this.onNewThumbnail.raise({ key: event.key, thumb: thumb });
            });
        }
    }

`Thumbnailer` accepts two `coconut.x.Bucket`s in its constructor, subscribes to the source's `onNewObject` event,
creates new thumbnails in the resulting lambda, and stores them in the other bucket.

It is important to note that the body of this lambda is real JavaScript -- and can use libraries from NPM, perform IO
and `await`s, etc. -- while the configuration outside of it is the CocoJS subset.  Notice how we can mix what
would have been classically expressed using a combination of configuration and real programming languages in one
consistent and idiomatic programming model.  This illustrates why Coconut's multi-language capabilities are important.

Also notice that `Thumbnailer` exposes its own event, `onNewThumbnail`, that can be subscribed just as it
subscribes to bucket events.  This enables an extensible ecosystem of events and handlers.  These events and handlers
are enlightened when targeting clouds that support first class events and handlers (such as S3 buckets and AWS Lambdas).

The result is a reusable cloud component that can be instantiated any number of times in any number of environments.

In fact, let us now look at code that uses `Thumbnailer`:

    import * as aws from "@coconut/aws";
    import {Thumbnailer} from "...";

    let images = new aws.s3.Bucket("images");
    let thumbnails = new aws.s3.Bucket("thumbnails");
    let thumbnailer = new Thumbnailer(images, thumbnails);

This package is an executable because it is meant to be run directly to create a new cloud topology.  Many Coconut
programs are libraries (like `Thumbnailer` itself), while blueprints are akin to executables in your favorite language.

The `aws.s3.Bucket` class is a subclass of `coconut.x.Bucket`, and so can be passed to `Thumbnailer`'s constructor just
fine.  We could have passed an `azure.blob.Container`, `google.storage.Bucket`, or a custom subclass, instead.  Notice
how `Thumbnailer` is itself a cloud-neutral abstraction.  Of course, if it had wanted to access specific AWS S3
features, it could have requested a concrete `aws.s3.Bucket` instead; or it can enlighten itself and use advanced
features simply by using a runtime type check; etc.  As with ordinary object-oriented languages, the class author
decides.  In fact, this example is remarkably similar to accepting a concrete "list" versus "enumerable" interface.

## Architecture

The primary concepts in Coconut are:

* **Package**: A static library or executable containing modules, classes, functions, and variables.
* **Resource**: A special kind of class that represents a cloud resource (VM, VPC, subnet, bucket, etc).
* **Environment**: A target environment with a name and optional configuration (e.g., production, staging, etc).
* **Stack**: An instantiation of a package, paired with an environment, and fully specified with arguments.
* **Resource URN**: An ID that is auto-assigned to each resource object, unique within the overall environment.

Analagous to programming languages, a stack is essentially a collection of instantiated resource objects.  Many
concepts that are "distinct" in other systems, like gateways, controllers, functions, triggers, and so on, are expressed
as classes in Coconut.  They are essentially "subclasses" -- or specializations -- of the more general concept of a
resource object, unifying the creation, configuration, provisioning, discovery, and overall management of them.

There are some different kinds of stacks we will encounter:

* **Plan**: A hypothetical stack created for purposes of inspection but that has not been deployed yet.
* **Deployment**: A stack that has actually been deployed into an environment.
* **Snapshot**: A stack generated by inspecting a live environment, not necessarily deployed using Coconut.

Notice that Coconut has the ability to generate programs and stacks from an existing environment.  This can be useful
during the initial adoption of Coconut.  It can also be useful for ongoing drift analysis, such as ensuring resources in
production don't differ from staging, that resources in different regions are equivalent, and/or verifying that changes
to an environment haven't been made without corresponding changes being made and checked into the Coconut metadata.

In addition to those core abstractions, there are some supporting ones:

* **Identity**: A unit of authentication and authorization, governing access to protected resources.
* **Configuration**: A bag of key/value settings used either at build, runtime, or a combination.
* **Secret**: A special kind of key/value configuration bag that is encrypted and protected by identity.

There are some other "internal" concepts that most users can safely ignore:

* **CocoLang**: A language subet for the Coconut configuration system (e.g., CocoJS, CocoPy, CocoGo, etc).
* **CocoPack**: The intermediate package format used as metadata for packages, common to all languages.
* **CocoIL**: The intermediate language (IL) used to represent code and data within a CocoPack package.
* **CocoGL**: The graph language (GL) describing a stack's resource topology with dependencies as edges.

Because Coconut is a tool for interacting with existing clouds -- including AWS, Azure, Google Cloud, Kubernetes, and
Docker Swarm -- one of the toolchain's most important jobs is faithfully mapping abstractions onto "lower level"
infrastructure.  Much of Coconut's ability to deliver on its promise of better productivity, sharing, and reuse relies
on its ability to robustly and intuitively perform these translations.  There is an extensible provider model for
creating new providers, which amounts to implementing create, read, update, and delete (CRUD) methods per resource type.

## Toolchain

The Coconut toolchain analyzes programs and understands them fully.  An important thing to realize is that a Coconut
program isn't "run" directly in the usual way; instead, the process is as follows:

* The source program is compiled (by a CocoLang compiler) into a package (a CocoPack containing CocoIL).
* The package itself may depend on any number of other library packages (themselves just CocoPacks).
* The executable package (and its CocoIL) is evaluated by the Coconut runtime to produce a graph (in CocoGL).
* This graph is then used to generate a plan, deployment, or simply output that can be inspected by a tool.

In particular, deployments are performed by the tool diffing the current state of an environment with the proposed new
state, to come up with a series of edits to make the new state reality.  This results in a series of invocations to
the necessary resource providers that perform side-effects that yield the necessary physical changes.

For instance, a standard workflow for creating a new project might look like this.

First, we initialize the project.  This isn't special, other than a `Coconut.json` or `.yaml` file:

    $ mkdir acmecorp-infra && cd acmecorp-infra   # create a project directory
    $ ...                                         # create a Coconut.* file, edit code, etc.

Next, we might choose to compile the project without performing a deployment.  This is used for inner loop development,
and reports any compile-time errors, while producing a package that we can distribute or inspect.

    $ coco compile                                # compile the project

Now we are ready to do a deployment.  First, we will initialize a target environment.  Let's call it `test`:
    
    $ coco env init test                          # initialize a test environment
    $ coco env config test ...                    # configure the target environment

The configuration steps are optional, but may be used to configure the target region, credentials, and so on.

Next, we might choose to do a dry-run of a deployment first (a.k.a., create a "plan"):

    $ coco deploy test -n                         # do a dry-run (plan) of the changes
	Planned step #1 [create]
	+ aws:ec2/instance:Instance:
          [urn=coconut:test::ec2instance:index::aws:ec2/instance:Instance::instance]
          imageId       : "ami-f6035893"
          instanceType  : "t2.micro"
          name          : "instance"
          resource      : "AWS::EC2::Instance"
          securityGroups: [
              [0]: -> *urn:coconut:test::ec2instance:index::aws:ec2/securityGroup:SecurityGroup::group
          ]
	1 planned changes:
		+ 1 resource created

This shows a colorized view of the series of changes that will be carried out.  The output is meant to resemble a
familiar Git-like diff view.  This will include adds (green), deletes (red), and modifications (yellow).

Finally, we would presumably perform the deployment.  This is done simply by omitting the `-n` flag:

    $ coco deploy test                            # actually perform the deployment

The output of an actual deployment will look a lot like the plan output above, except that it contains more incremental
information about the status of steps as they are taken.  For example:

	Applying step #1 [create]
	+ aws:ec2/instance:Instance:
			  [urn=test::ec2instance:index::aws:ec2/instance:Instance::instance]
			  imageId       : "ami-f6035893"
			  instanceType  : "t2.micro"
			  name          : "instance"
			  resource      : "AWS::EC2::Instance"
			  securityGroups: [
				  [0]: -> *urn:coconut:test::ec2instance:index::aws:ec2/securityGroup:SecurityGroup::group
			  ]
	info: plugin[aws].stdout: Creating new EC2 instance resource
	info: plugin[aws].stdout: EC2 instance 'i-0c5192a1d67810e1a' created; now waiting for it to become 'running'
	1 total changes:
		+ 1 resource created

Subsequent changes may be made in the expected way:

    $ ...                                         # more code edits, etc.
    $ coco deploy test                            # re-deploy (automatically recompiles)

Coconut calculates the minimal set of incremental edits, compared to the previous deployment, so that just the changed
parts will be modified in the target environment.

## Further Reading

More details are left to the respective design documents.  Here are some key ones:

* [**Formats**](design/formats.md): An overview of Coconut's three formats: CocoLangs, CocoPack/CocoIL, and CocoGL.
* [**CocoPack/CocoIL**](design/packages.md): A detailed description of packages and the CocoPack/CocoIL formats.
* [**Dependencies**](design/deps.md): An overview of how package management and dependency management works.
* [**Resources**](design/resources.md): A description of how extensible resource providers are authored and registered.
* [**CocoGL**](design/graphs.md): An overview of the CocoGL file format and how Coconut uses graphs for deployments.
* [**Stacks**](design/stacks.md): An overview of how stacks are represented using the above fundamentals.
* [**Clouds**](design/clouds.md): A description of how Coconut abstractions map to different cloud providers.
* [**Runtime**](design/runtime.md): An overview of Coconut's runtime footprint and services common to all clouds.
* [**Cross-Cloud**](design/x-cloud.md): An overview of how Coconut can be used to create cloud-neutral abstractions.
* [**Security**](design/security.md): An overview of Coconut's security model, including identity and group management.
* [**FAQ**](faq.md): Frequently asked questions, including how Coconut differs from its primary competition.

