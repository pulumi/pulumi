# Coconut Overview

Coconut is a toolset and runtime for creating reusable cloud services.  It lets you author packages that can be shared
and consumed just like your favorite programming language's libraries.  Coconut is inherently multi-language and
multi-cloud, and lets you build abstractions that span different cloud environments and topologies, if you desire.

This document provides an overview of the Coconut system, its goals, primary concepts, and the system architecture.

## Problem

Cloud services are complex.  They are complex to build, complex to deploy, and complex to manage.  The current trend to
use increasingly fine-grained microservices simply exacerbates this complexity, transforming most modern cloud
applications into complex distributed systems.

There are many aspects to building distributed systems that aren't evident to the newcomer -- like RPC, logging,
fault tolerance, and zero-downtime deployments -- and even the experienced practitioner will quickly find that the
developer and operations tools do not guide us down the golden path.  Indeed, it is common to need to master a dozen
tools before even getting an application up and running in production.

On top of that complexity, it is difficult to share knowledge.  Most modern programming languages have component models
that allow you to bundle up complex functionality underneath simple abstractions, and package managers that allow you
to share these components with others, and consume components shared by others, in the form of libraries.  The current
way that cloud architectures are built and deployed is simply not amenable to this kind of sharing.

Even if there was a way to share components, the cloud platforms are divergent in the configuration languages they
accept, infrastructure abstractions that they provide, and the specific knobs used to configure those abstractions.  It
is as though every Node.js programmer out there needs to understand the intricate details of the Linux scheduler, versus
macOS, versus Windows, just to deliver a simple web application.

Finally, once such an application is up and running, managing and evolving it requires similarly ad-hoc and
individualized tools and practices.  Applying changes to a running environment is often done manually, in a
hard-to-audit way, and patches are applied unevenly and inconsistently, often leaving security problems open to attack.

All of the above is a big productivity drain, negatively impacting the agility of organizations that need to innovate
in service of their businesses.  It means that improvements are more costly to deliver.  Containers have delivered a
great improvement to the management of single nodes in a cluster, but has not yet expanded that same simplicity to
managing entire applications or entire clusters.  Thankfully by adopting concepts and ideas that have worked in the
overall landscape of languages and runtimes, we can make considerable improvements in all of these dimensions.

## Solution

Coconut lets developers author components in their language of choice (JavaScript, Python, Ruby, Go, etc).  This is in
contrast to most cloud programming models today which require the use of often-obscure configuration languages or DSLs.
This unified view is particularly helpful when building serverless applications where you want to focus on code.

At the same time, Coconut is polyglot, allowing composition of components authored in many different languages.

These components can be built by reusing existing components shared by others, and published to the Coconut package
manager for others to use.  Cloud services are simply instances of these components, with property values configured
appropriately, and change management is done automatically by Coconut's understanding of the overall graph of
dependencies between those services.  Think of each service as an "object" that is running in the cloud.

Coconut runs on any public or private cloud.  Although you are free to program directly to your cloud provider's
specific abstractions, using the full power of your native cloud, Coconut also facilitates building higher-level
cloud-neutral components that can run anywhere. This includes compute services, storage services, and even more logical
domain-specific services like AI, ML, and recognition.

## Concepts

The primary concepts in Coconut are:

* **Stack**: A static description of a topology of cloud services with optional APIs.
* **Package**: A collection of exports stacks for consumption by others.
* **Service**: An instantiation of a stack, grouping zero to many services, together.
* **Cluster**: A hosting environment that stacks can be deployed into, reifying them as services.
* **Workspace**: A static collection of zero to many stacks managed together in a single source repository.

In an analogy with programming languages, a stack is like a *class* and a service is like an *object*.  Many concepts
that are "distinct" in other systems, like the notion of gateways, controllers, functions, triggers, and so on, are
expressed as stacks and services in the Coconut system.  They are essentially "subclasses" -- or specializations -- of
this more general concept, unifying the configuration, provisioning, discovery, and overall management of them.

In addition to those core abstractions, there are some supporting ones:

* **Identity**: A unit of authentication and authorization, governing access to protected resources.
* **Configuration**: A bag of key/value settings used either at build, runtime, or a combination.
* **Secret**: A special kind of key/value configuration bag that is encrypted and protected by identity.

Because Coconut is a tool for interacting with existing clouds -- including targets like AWS, Kubernetes, and Docker
Swarm -- one of the toolchain's most important jobs is faithfully mapping these abstractions onto "lower level"
infrastructure abstractions, and vice versa.  Much of Coconut's ability to deliver on its promise of better
productivity, sharing, and reuse relies on its ability to robustly and intuitively perform these translations.  There
is an extensible provider model for creating new providers, which amounts to implementing create, read, update, and
delete (CRUD) methods per resource type.

## Example

Let us look at an example stack written in Coconut's flavor of JavaScript, CocoJS:

    import * as coconut from "@coconut/coconut";

    export class Thumbnailer extends coconut.Stack {
        private source: coconut.Bucket; // the source to monitor for images.
        private dest: coconut.Bucket;   // the destination to store thumbnails in.

        constructor(source: coconut.Bucket, dest: coconut.Bucket) {
            this.source = source;
            this.dest = dest;
            this.source.onObjectCreated(async (event) => {
                let obj = await event.GetObject();
                let thumb = await gm(obj.Data).thumbnail();
                await this.dest.PutObject(thumb);
            });
        }
    }

This `Thumbnailer` stack simply accepts two `coconut.Bucket`s in its constructor, stores them, and wires up a lambda to
run on the source's `onObjectCreated` event.  This program describes a reusable cloud service that can be instantiated
any number of times in any environment.  It is important to note that the body of this lambda is real JavaScript, while
the configuration outside of it is the CocoJS subset.  Note how we can mix what would have been classically expressed
using a combination of configuration and real programming languages in one consistent and idiomatic programming model.

Let us now look at an instantiation of `Thumbnailer`.  This happens elsewhere in something we call a *blueprint*:

    import * as aws from "@coconut/aws";
    import {Thumbnailer} from "...";

    let images = new aws.s3.Bucket("images");
    let thumbnails = new aws.s3.Bucket("thumbnails");
    let thumbnailer = new Thumbnailer(images, thumbnails);

Many Coconut programs are libraries, while blueprints are akin to executables in your favorite language.

The `aws.s3.Bucket` class is a subclass of `coconut.Bucket`, and so can be passed to `Thumbnailer`'s constructor just
fine.  Notice how `Thumbnailer` is itself a cloud-neutral abstraction.  Of course, if it had wanted to access specific
AWS S3 features, it could have accepted a concrete `aws.s3.Bucket`; as with ordinary object-oriented languages, the
abstraction's author decides (e.g., this is similar to accepting a concrete "list" versus "enumerable" interface).

The Coconut toolchain analyzes this program and understands its components.  The program isn't run directly; instead, it
is fed into a command like `coco compile`, `coco plan`, and `coco apply`, to determine how to create, update, or delete
resources in a target cluster environment, using resource providers.  For instance, running `coco apply` on the above
program, in a new cluster, will create two S3 buckets and a single AWS lambda wired up to the source bucket.  If we make
edits, and reapply those edits, just the parts that have been changed will be updated in the target environment.

## Further Reading

More details are left to the respective design documents.  Here are some key ones:

* [**Formats**](design/formats.md): An overview of Coconut's three formats: CocoLangs, NutPack/NutIL, and CocoGL.
* [**NutPack/NutIL**](design/nutpack.md): A detailed description of Nuts and the NutPack/NuIL formats.
* [**CocoGL**](design/cocogl.md): An overview of the CocoGL file format and how Coconut uses graphs for deployments.
* [**Stacks**](design/stacks.md): An overview of how stacks are represented using the above fundamentals.
* [**Dependencies**](design/deps.md): An overview of how package management and dependency management works.
* [**Clouds**](design/clouds.md): A description of how Coconut abstractions map to different cloud providers.
* [**Runtime**](design/runtime.md): An overview of Coconut's runtime footprint and services common to all clouds.
* [**Cross-Cloud**](design/x-cloud.md): An overview of how Coconut can be used to create cloud-neutral abstractions.
* [**Security**](design/security.md): An overview of Coconut's security model, including identity and group management.
* [**Resources**](design/resources.md): A description of how extensible resource providers are authored and registered.
* [**FAQ**](faq.md): Frequently asked questions, including how Coconut differs from its primary competition.

