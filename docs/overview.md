# Mu Overview

Mu is a toolset and runtime for creating reusable cloud services.  Mu lets you author packages that can be shared and
consumed just like your favorite programming language's libraries.  Mu is inherently multi-language, multi-cloud, and
supports building abstractions that span different cloud environments and topologies.

This document provides an overview of the Mu system, its goals, primary concepts, and the system architecture.

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

Mu lets developers author components in their language of choice (JavaScript, Python, Ruby, Go, etc).  This is in
contrast to most cloud programming models today which require the use of often-obscure configuration languages or DSLs.
This unified view is particularly helpful when building serverless applications where you want to focus on code.

At the same time, Mu is polyglot, allowing composition of components authored in many different languages.

These components can be built by reusing existing components shared by others, and published to the Mu package manager
for others to use.  Cloud services are simply instances of these components, with property values configured
appropriately, and change management is done automatically by Mu's understanding of the overall graph of dependencies
between those services.  Think of each service as an "object" that is running in the cloud.

Mu runs on any public or private cloud.  Although you are free to program directly to your cloud provider's specific
abstractions, Mu also facilitates building more abstract cloud-neutral components that can run anywhere.  This includes
compute services, storage services, and even more logical domain-specific services like AI, ML, and recognition.

## Concepts

The primary concepts in Mu are:

* **Stack**: A static description of a topology of cloud services with optional APIs.
* **Package**: A collection of exports stacks for consumption by others.
* **Service**: An instantiation of a stack, grouping zero to many services, together.
* **Cluster**: A hosting environment that stacks can be deployed into, reifying them as services.
* **Workspace**: A static collection of zero to many stacks managed together in a single source repository.

In an analogy with programming languages, a stack is like a *class* and a service is like an *object*.  Many concepts
that are "distinct" in other systems, like the notion of gateways, controllers, functions, triggers, and so on, are
expressed as stacks and services in the Mu system.  They are essentially "subclasses" -- or specializations -- of this
more general concept, unifying the configuration, provisioning, discovery, and overall management of them.

In addition to those core abstractions, there are some supporting ones:

* **Identity**: A unit of authentication and authorization, governing access to protected resources.
* **Configuration**: A bag of key/value settings used either at build, runtime, or a combination.
* **Secret**: A special kind of key/value configuration bag that is encrypted and protected by identity.

Because Mu is a tool for interacting with existing clouds -- including targets like AWS, Kubernetes, and Docker Swarm --
one of the toolchain's most important jobs is faithfully mapping these abstractions onto "lower level" infrastructure
abstractions, and vice versa.  Much of Mu's ability to deliver on its promise of better productivity, sharing, and reuse
relies on its ability to robustly and intuitively perform these translations.  There is an extensible provider model for
creating new providers, which amounts to implementing create, read, update, and delete (CRUD) methods per resource type.

## Example

Let us look at an example stack written in Mu's flavor of JavaScript, MuJS:

    import * as mu from "mu";

    export class Thumbnailer extends mu.Stack {
        private source: mu.Bucket; // the source to monitor for images.
        private dest: mu.Bucket;   // the destination to store thumbnails in.

        constructor(source: mu.Bucket, dest: mu.Bucket) {
            this.source = source;
            this.dest = dest;
            this.source.onObjectCreated(async (event) => {
                let obj = await event.GetObject();
                let thumb = await gm(obj.Data).thumbnail();
                await this.dest.PutObject(thumb);
            });
        }
    }

This `Thumbnailer` stack simply accepts two `mu.Bucket`s in its constructor, stores them, and wires up a lambda to run
on the source's `onObjectCreated` event.  This program describes a reusable cloud service that can be instantiated any
number of times in any environment.  It is important to note that the body of this lambda is real JavaScript, while the
configuration outside of it is the MuJS subset.  Mu is letting us mix what would have been classically expressed using a
combination of configuration and real programming languages in one consistent and idiomatic programming model.

Let us now look at an instantiation of `Thumbnailer`.  This happens elsewhere in something we call a *blueprint*:

    import * as aws from "@mu/aws";
    import * as mu from "mu";
    import {Thumbnailer} from "...";

    let images = new aws.s3.Bucket("images");
    let thumbnails = new aws.s3.Bucket("thumbnails");
    let thumbnailer = new Thumbnailer(images, thumbnails);

Many Mu programs are libraries, while blueprints are akin to executables in your favorite language.

The `aws.s3.Bucket` class is a subclass of `mu.Bucket`, and so can be passed to `Thumbnailer`'s constructor just fine.
Notice how `Thumbnailer` is itself a cloud-neutral abstraction.  Of course, if it had wanted to access specific AWS S3
features, it could have accepted a concrete `aws.s3.Bucket`; as with ordinary object-oriented languages, the
abstraction's author decides (e.g., this is similar to accepting a concrete "list" versus "enumerable" interface).

The Mu toolchain analyzes this program and understands its components.  The program isn't run directly; instead, it is
fed into a command like `mu compile`, `mu plan`, and `mu apply`, to determine how to create, update, or delete resources
in a target cluster environment, using resource providers.  For instance, running `mu apply` on the above program, in a
new cluster, will create two S3 buckets and a single AWS lambda wired up to the source bucket.  If we make edits, and
reapply those edits, just the parts that have been changed will be updated in the target environment.

## Further Reading

More details are left to the respective design documents.  Here are some key ones:

* [**Languages**](design/languages.md): An overview of Mu's three languages: MetaMus, MuPack/MuIL, and MuGL.
* [**MuPack/MuIL**](design/mupack.md): A detailed description of Mu's packaging and computation formats.
* [**MuGL**](design/mugl.md): An overview of the MuGL file format and how Mu uses graphs to do deployments.
* [**Stacks**](design/stacks.md): An overview of how stacks are represented using the above fundamentals.
* [**Dependencies**](design/deps.md): An overview of how package management and dependency management works.
* [**Clouds**](design/clouds.md): A description of how Mu abstractions map to different cloud providers.
* [**Runtime**](design/runtime.md): An overview of Mu's runtime footprint and services common to all clouds.
* [**Cross-Cloud**](design/x-cloud.md): A brief description of how Mu can be used to create cloud-neutral abstractions.
* [**Security**](design/security.md): An overview of Mu's security model, including identity and group management.
* [**Resources**](design/resources.md): A description of how extensible resource providers are authored and registered.
* [**FAQ**](faq.md): Frequently asked questions, including how Mu differs from its primary competition.

