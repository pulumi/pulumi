# Coconut Stacks

This document describes how Coconut Stacks and Services show up in the various [formats](formats.md) (CocoLangs,
NutPack, NutIL, and CocoGL).  Those are the definitive resources on the low-level formats, but this document describes
the overall programming model that a developer will encounter.  For more details on how this results in concrete
resources provisioned in the target cloud provider, please refer to [the cloud targeting design document](clouds.md).

## Overview

The following are the basic steps to creating a new NutPack:

* Pick your favorite CocoLang language.
* Create a new project folder in your workspace.
* Create a Nutfile (`Nut.yaml`) containing the top-level metadata.
* Install any dependencies using the `coco get` command line.
* Author stacks by subclassing the `Stack` base class in the Coconut SDK.
* Build the package using `coco build`, rinse and repeat, and then publish it.

For illustration purposes within this document, we shall choose Coconut's JavaScript subset, CocoJS.  Please also note
that, though metadata examples are in YAML, it is generally valid to use JSON instead if preferred.

TODO: this document needs some good examples!

## Metadata

Most NutPacks will contain a Nutfile to help direct the compilation process.  It is conventionally named `Nut.yaml` and
is checked into the workspace.

Each Nutfile contains metadata for the package that cannot be derived from the source code.  (Please refer to
[the NutPack document](nutpack.md) for a complete listing of what metadata can appear here.)  In the case that all
metadata can be derived from the program alone -- e.g., thanks to the use of attributes/decorators -- then the Nutfile
might be omitted.  This is specific to your CocoLang compiler, so please consult its documentation.

In the case of CocoJS, and most CocoLang compilers, the top-level "package manager"-like metadata -- such as name,
description, and so on -- must be explicitly provided; for example:

    name: acmecorp/elk
    description: A fully functioning ELK stack (Elasticsearch, Logstash, Kibana).
    author: Joe Smith <joesmith@acmecorp.com>
    website: https://acmecorp.github.io/elk
    keywords: [ elasticsearch, logstash, kibana ]

In addition to basic metadata like this, any dependency packages must also be listed explicitly:

    dependencies:
        - aws/ec2#^1.0.6

Each dependency package should consist of the following elements:

* An optional protocol (e.g., `https://`).
* An optional base URL (e.g., `cocohub.com/`, `github.com/`, etc).
* A required namespace and/or name part (e.g., `acmecorp/worker`, `aws/s3/bucket`, etc).
* An optional `#` followed by version number (e.g., `#^1.0.6`, `#6f99088`, `#latest`, etc).

If protocol and base URL are absent, Coconut will default to `https://cocohub.com/`.  If the version is omitted, it will
default to `latest`, which just means "tip"; in other words, the most recent version is used at compile-time.

Please refer to the [dependencies design document](deps.md) for details on the format for these references in addition
to the overall package resolution process.

TODO: it's unclear where and how [security information](security.md) should appear.

## Defining Stacks

As we saw above, a stack is any subclass of Coconut's base `Stack` class:

    export class Registry extends coconut.Stack {
        private table: coconut.Table;

        constructor() {
            this.table = new coconut.Table("names");
        }

        @api public register(name: string): Promise<number> {
            return this.table.insert({ name: name });
        }
    }

Any additional stacks instantiated by this stack will get transformed into CocoGL services at planning and deployment
time.  The `Registry` above is very simple, since it doesn't accept any properties or constructor arguments, and doesn't
expose any properties of its own.  But clearly each could be an interesting extension.  We will see examples shortly.

A stack is capable of representing many different kinds of cloud "services": infrastructure, databases, containers,
event-oriented systems, web service applications, microservices, SaaS dependencies, ..., and so on.  This consistency
facilitates a degree of composition and reuse, plus consistency in the way they are created, configured, and managed.

### Subclassing

A stack may subclass any other stack, specializing aspects of it as appropriate.  This facilitates reuse.  For instance,
perhaps my company wishes to enforce that certain best practices and standards are adhered to, for all stacks.  Or
imagine someone in the community has published a best-in-breed Node.js application blueprint, leveraging Express,
MongoDB, and the ELK stack, and I merely want to plug in my own application logic and leverage the overall Stack.

### Capabilities

Any reference to a service instance is called a "capability".

A capability is an unforgeable reference to another running service and can be used to interact with it, either through
configuration, RPC, or otherwise.  By defining interfaces in terms of capabilities, we enable a more formal way of
expressing runtime dependencies, in a way that the system can understand and leverage in its management of the system
(like ensuring services are created in the right order).

The more statically typed approach of using service capabilities also eliminates some of the fragility common to weakly
typed and dynamic approaches, which can be prone to race conditions, requiring manual sleeps, retries, etc.

Capabilities can also benefit from the abstraction and encapsulation provided by Coconut.  For example, imagine we want
a key/value store.  The `coconut/x` namespace offers such a `KVStore` abstraction, but it is abstract.  By declaring in
a constructor that we require a `KVStore`, we leave open the possibility that a caller might provide an instance of
etcd, Consul, Zookeeper, or their favorite key/value store provider.

The references between services forms a DAG and the system topologically sorts that DAG in order to determine the order
in which to create and destroy services, during CocoGL planning time.  There must be no cycles.  Resource providers
understand liveness and health, so that developers needn't worry about races, liveness, or retries in CocoLang code.

### Exporting Services

By default, services created by a stack are private implementation details of the enclosing service definition.  It is
possible to export instances for public usage as a capability, however, simply by assigning them to output properties.
After constructing a service with outputs, they will be available for read access by callers.

### RPC

Every stack may choose to expose protocols.  These can be standard "unconstrained" network interfaces, such as "HTTP
over port 80", or can take on a more structured form, like structured RPC or REST APIs.

The above example demonstrates this.  The `@api` annotation above makes the `register` function available as an API
at runtime.  This is convenient because we can simply close over `this.table` to reference it at runtime, versus the
common practice of keeping configuration and code separate, and then needing to use "loose binding" through a
combination of dynamic lookup, environment variable-based configuration, manual establishment of channels, etc.

The benefits of declaring the full interfaces are that the RPC semantics are known to the system, facilitating advanced
management capabilities such as diagnostics, monitoring, fuzzing, and self-documentation, in addition to RPC code-
generation.  This also adds a sort of "strong typing" to the connections between stacks.

In addition to `@api`, the `@event` annotation lets a stack export an event.  This event can be subscribed to and used
to schedule serverless lambda invocations, among other things.  All events are subject to the same restrictions as APIs.

All RPC functions must deal solely in terms of simple schema types on the wire, since they map to HTTP/2-based RPC and
Internet protocols.  Please refer to the [RPC design document](rpc.md) for details on how this works.

### Readonly and Perturbing Properties

A readonly property cannot be changed after provisioning a resource without replacing it.

This is often used by core "infrastructure" that cannot change certain properties after creation, for example, the
data-center, virtual private cloud, or physical machine size.  Although the tools allow you to change these, the mental
model is that of creating a "new" object, and wiring up all of its dependencies all over again.  As a result, the
deployment process is more delicate, and may trigger a cascading recreation of many resources.

How a readonly property is expressed is CocoLang language-specific, however for languages like CocoJS that support a
`readonly` property modifier, that is how it's done.

A "perturbing" property is one that can be changed after provisioning, but doing so requires perturbing the existing
service in a way that may interrupt the live service.  Modifying this isn't quite as impactful to the deployment process
as modifying a readonly property, however it too must be treated with care.

TODO(joe): CloudFormation distinguishes between three modes: update w/out interruption, update w/ interruption, and
replacement; I personally like the logical nature of readonly, however it's possible we should adopt something closer to
it: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-update-behaviors.html.

## Configuration

TODO(joe): write this section.

## Common Stack Types

Coconut offers a complete set of infrastructure stacks for each cloud provider.

Coconut also provides the `coconut/x` package, which contains a set of logical stack types, like `Container`, `Lambda`,
`Table`, `Volume`, and so on, offering a framework of higher-level, cloud-agnostic abstractions.  Please consult
[the cross-cloud design document](x-cloud.md) for more details on this package and its contents.

