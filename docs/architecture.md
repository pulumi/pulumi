# Mu Architecture

Mu introduces abstractions that help developers create, maintain, and reason about microservice-based architectures.
More than that, Mu facilitates sharing and reuse of these architectures between developers and teams.

The three top-level architectural abstractions in Mu are:

* **Stack**: A static blueprint, or "type", that describes a topology of cloud resources.
* **Service**: An instantiation of a Stack, grouping zero to many services, each with an optional API, together.
* **Workspace**: A static collection of one or many Stacks managed together in a single source repository.

In an analogy with object-oriented systems, a Stack is akin to a class and Service is akin to an object.

Many concepts that are "distinct" in other systems, like the notion of Gateways, Controllers, Functions, Triggers, and
so on, are expressed as Stacks and Services in the Mu system.  They are essentially "subclasses" -- or specializations
-- of this more general concept, unifying the configuration, provisioning, discovery, and overall management of them.

Because Mu is a tool for interacting with existing clouds -- including targets like AWS, Kubernetes, and Docker Swarm --
one of the toolchain's most important jobs is faithfully mapping these abstractions onto "lower level" infrastructure
abstractions, and vice versa.  Much of Mu's ability to deliver on its promise of better productivity, sharing, and reuse
relies on its ability to robustly and intuitively perform these translations.

This document describes the overall architecture for the system, including this translation process.

## Platform

A Stack is comprised of many Services, instantiated from other Stacks.  This of course eventually bottoms out on a core
set of "primitive" constructs, offered by the Mu platform.  In addition to those primitives, a rich ecosystem exists on
top, so that, for the most part, end-user Stacks seldom need to interact directly even with the primitives.

The primitive types available include:

* `mu/container`: A Docker container wrapped in Mu metadata.
* `mu/gateway`: An API gateway and/or load balancer, multiplexing requests onto multiple target Services.
* `mu/func`: A single Function ordinarily used for serverless/stateless scenarios.
* `mu/event`: An Event that may be used to Trigger the execution of another Service (commonly a Function).
* `mu/volume`: A volume stores data that can be mounted by another Service.
* `mu/autoscaler`: A Service that automatically multi-instances and scales some other target Service based on policy.
* `mu/hook`: A logical Service that has no concrete runtime manifestation other than running pre- or post-logic.

TODO(joe): link to exhaustive details on each of these.

TODO(joe): is there an extensibility model, whereby the set of primitive constructs can be extended?

## Metadata Specification

The essential artifact a developer uses to create Stacks and Services is something we call a Mufile.  It is
conventionally named `Mu.yaml`, usually checked into the Workspace, and each single file describes a Stack.  Note that
Stacks may internally contain other Stacks, however this is purely an implementation detail of how the Stack works.

The full Mufile format is specified in [TODO](/TODO).  However, we present a brief overview here.

### Package Manager Metadata

Each Mufile begins with some standard "package manager"-like metadata, like name, version, description, and so on.  As
with most package managers, most of these elements are optional:

    name: elk
    version: 1.0.1
    description: A fully functioning ELK stack (Elasticsearch, Logstash, Kibana).
    author: Joe Smith <joesmith@elk.com>
    website: https://github.com/joesmith/elk

TODO(joe): finish this section.

### Services

After that comes the section that describes what Services make up this Stack:

    services:

At this point, a new concept is introduced: *visibility*.  Visibility works much like your favorite programming
language, in that a Stack may declare that any of its Services are `public` or `private`.  This impacts the
accessibility of those Services to consumers of this Stack.  A private Service is merely an implementation detail of
the Stack, whereas a public one is actually part of its outward facing interface.  This facilitates encapsulation.

For instance, perhaps we are leveraging an S3 bucket to store some data in one of our Services.  That obviously
shouldn't be of interest to consumers of our Stack.  So, we split things accordingly:

    services:
        private:
            aws/s3:
                bucket: images
        public:
            nginx/nginx:
                data: s3
                port: 80

In this example, S3 buckets are volumes; we create a private one and mount it in our public Nginx container.

TODO(joe): more examples.

TODO(joe): a section on names, scoping, etc.

### Nested Stacks

Another feature that comes in handy sometimes is the ability to create nested Stacks:

    stacks:

Each nested Stack is very much like the Stack defined by any given Mufile, except that it is scoped, much like a
nested/inner class in object-oriented languages.  There are two primary reasons you might do this:

1. Multi-instancing that Stack, to create multiple Services.
2. Conveniently leveraging that Stack as the "type" for one or more other Services in the file.

TODO(joe): examples of the above.

TODO(joe): we need to decide whether you can export public Stacks for public consumption.  At this point, my stance is
that you must create an entirely different Stack to do that.  This keeps things simple for the time being.

## An Illustrative Example

Before breaking down the implementation of these concepts, let's first look at an illustrative example.

TODO(joe): a more comprehensive example (like Vote50) that illustrates more of these concepts working in harmony.

## Implementation

In this section, we will breakdown the primary aspects of the implementation:

1. Translation

TODO(joe): deployment, ongoing interactions, management, etc.

### Translation

Now let us look at how a Mufile turns into a ready-to-run package.

In the future, we envision that Mufiles might be generated from code, for an even more seamless developer experience.
This is very powerful when envisioning serverless architectures, where Stacks, Services, Functions, and Triggers can be
expressed all in a single file, and managed alongside the code and libraries they depend upon.  See marapongo/mu#xxx
for a work item tracking this.  For now, and to simplify this doc, we will ignore this possibility.

TODO(joe): write this section; just cover the overall structure and plugin model, not the details of each target.

TODO(joe): create a design specification that covers the detailed translation for each target; link to it.

