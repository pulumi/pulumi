# Mu Metadata Specification

This document contains a formal description of Mu's metadata.  For more details on how this metdata is compiled when
targeting various cloud providers, please refer to [the companion design document](targets.md).

## Overview

The essential artifact a developer uses to create Stacks and Services is something we call a Mufile.  It is
conventionally named `Mu.yaml`, usually checked into the Workspace, and each single file describes a Stack.  (Note that
Stacks may internally contain other Stacks, however this is purely an implementation detail of how the Stack works.)

TODO(joe): declarative specification format for Clusters.

Although all examples are in YAML, it is perfectly valid to use JSON instead if that is more desirable.

Mu preprocesses all metadata files to substitute context values not known until runtime, such as configuration,
arguments, and so on.  The [Go template syntax](https://golang.org/pkg/text/template/) is used for this.  Please refer
to the API documentation for the context object (TODO(joe): do this) for details on what information is available.

## Package Managament

Each Mufile begins with some standard "package manager"-like metadata, like name, version, description, and so on.  As
with most package managers, most of these elements are optional.  For example:

    name: elk
    version: 1.0.1
    description: A fully functioning ELK stack (Elasticsearch, Logstash, Kibana).
    author: Joe Smith <joesmith@elk.com>
    website: https://github.com/joesmith/elk

TODO(joe): full set of attributes.

In addition to basic metadata like this, any dependency packages must also be listed explicitly.

TODO(joe): finish this section.

## Security

TODO(joe): we need the ability to override the default Role/ACLs/etc.

## Stacks and Subclassing

A Stack may subclass any other Stack, specializing aspects of it as appropriate.  This facilitates reuse.  For instance,
perhaps my company wishes to enforce that certain best practices and standards are adhered to, for all Stacks.  Or
imagine someone in the community has published a best-in-breed Node.js application blueprint, leveraging Express,
MongoDB, and the ELK stack, and I merely want to plug in my own application logic and leverage the overall Stack.

To do this, reference another Stack's fully qualified name in the `base` property:

    base: some/other/stack

From there, I can specify additional metadata, however we will have inherited everything from the base.

TODO(joe): what about mixins?

TODO(joe): get more specific about what can be overridden.  Furthermore, what about "deletes"?

In addition to subclassing, a Stack may be marked abstract, indicating that it cannot be instantiated:

    abstract: true

A non-abstract Stack must in fact have a non-zero number of Services, whereas an abstract one can omit Services
entirely; this can be used, for example, to predefine certain non-Service metadata for subclassing Stacks.

## APIs

Every Stack may choose to export one or more APIs.  These APIs can be standard "unconstrained" network interfaces, such
as "HTTP over port 80", or can take on a more structured form, like leveraging OpenAPI to declare a protocol interface.
The benefits of declaring the full interfaces are that the RPC semantics are known to the system, facilitating advanced
management capabilities such as diagnostics, monitoring, fuzzing, and self-documentation, in addition to RPC code-
generation.  This also adds a sort of "strong typing" to the connections between Services.

TODO(joe): articulate this section further; e.g., the metadata format, precise advantages, etc.

## Stack Constructors

Each Stack can declare a set of properties that callers can set during creation:

    properties:

This is a bag of property names to property values, each of which has the following:

* `type`: A property type, restricting the legal values.
* `description`: An optional long-form description of the property.
* `default`: A default value to be supplied if the caller doesn't supply one.
* `optional`: If `true`, this property can be omitted at creation time.
* `readonly`: If `true`, this property cannot be set on a resource after provisioning, without recreating it.

For example:

    properties:
        title:
            type: string
            description: The title of this thing.
            default: Anonymous

### Types

The set of types a property may take on are "JSON-like".  This includes simple primitives:

    type: string
    type: number
    type: boolean
    type: object

As well as array shapes utilizing them:

    type: [ string ]
    type: [ number ]
    type: [ boolean ]
    type: [ object ]

Complex structures can be described simply using objects with properties:

    type:
        id: number
        name: string
        value: object

### Capability Types

The most interesting feature here is the ability to request a "capability", or reference to another Service.  This
provides a strongly typed and more formal way of expressing Service dependencies, in a way that the system can
understand and leverage in its management of the system (like ensuring Services are created in the right order).  It
also eliminates some of the fragility of weakly typed and dynamic approaches, which can be prone to race conditions.

The most basic form is to use the special type `service`:

    type: service

This is helpful, as it exposes a dependency to the system -- compared to dynamically discovering and depending on a name
at runtime -- but it isn't perfect.  The shape of the dependency is still opaque to the system.

Even better is to declare that we depend on a specific kind of Service, by specifying the fully qualified name of a
Stack.  In such a case, the system ensures an instance of this Stack type, or subclass, is provided:

    type: ex/kvstore

This hypothetical Stack defines an API that can be used as a key-value store.  Presumably we would find subclasses of it
for etcd, Consul, Zookeeper, and others, which a caller is free to choose from at instantiation time.

Another example leverages the primitive `mu/volume` type to require a Service which can be mounted as a volume:

    type: mu/volume

Note that anywhere inside of this Mufile, we may access the arguments supplied at Stack instantiation time using the Go
template syntax mentioned earlier.  For example, `{{.args.tag.name}}`.

### Readonly Properties

A readonly property is one that cannot be changed after provisioning a resource without replacing it.  This is often
used by core "infrastructure" that cannot change certain properties after creation, for example, the data-center,
virtual private cloud, or physical machine size.  Although the tools allow you to change these, the mental model is that
of creating a "new" object, and wiring up all of its dependencies all over again.  As a result, the deployment process
is more delicate, and may trigger a cascading recreation of many resources.

TODO(joe): CloudFormation distinguishes between three modes: update w/out interruption, update w/ interruption, and
replacement; I personally like the logical nature of readonly, however it's possible we should adopt something closer to
it: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/using-cfn-updating-stacks-update-behaviors.html.

## Configuration

TODO(joe): write this section.

## Services

After that comes the section that describes what Services make up this Stack:

    services:

In this section is zero-to-many Services that are co-created with one another.  Each Service has:

* A name, both for dynamic and static use.
* A type, which is just the name of a Stack to instantiate.
* A visibility governing whether consumers of this Stack have access to it or not.
* Any number of name/value properties, mapping to the Stack's settable properties.

Although these Services are co-created, they may reference one another.  The references between each other forms a DAG
and the system topologically sorts that DAG in order to determine the order in which to create and destroy Services.
Notably there may be no cycles.  By default, the system understands liveness and health (TODO(joe): how); as a result,
the developer need not explicitly worry about races, liveness, or retries during Service creation.

### Names

A Service's name can be set in one of two ways.  The simplest is to use the "default", derived from the Stack type.  For
example, in the following metadata, the single Service has type `nginx/nginx`, gets a default name of `nginx`:

    services:
        public:
            nginx/nginx:
                port: 80

Note that this is the latter part of the name; something called `elasticsearch/kibana` would get a name of `kibana`.

If we wish instead to give this an explicit name, say `www`, we can do so using the `type` property:

    services:
        public:
            www:
                type: nginx/nginx
                port: 80

A Service's name is visible at runtime (e.g., in logs, diagnostics commands, and so on), in addition to controlling how
metadata cross-referenes that Service.  All Services live within a Stack, which of course has a name.  Inside of a
Stack, this outer name becomes its Namespace.  For instance, inside of a Stack named `marapongo/mu`, a Service named `x`
has a fully qualified name (FQN) of `marapongo/mu/x`.  Although we seldom need the FQN for references within a single
Stack, they are sometimes needed for inter-Stack references, in addition to management activities.

### Types

Each Service has a type, which is merely the name of another Stack.  Most of the time this is the FQN, although for
references to other Stacks defined within the same Mufile (more on that later), this can just be a simple name.  During
instantiation of that Service, a fresh instance of that Stack is created and bound to in place of this Service name.

Although there are obviously many opportunities for ecosystems of user-defined Stacks, and indeed a rich library offered
by the Mu platofrm itself, we eventually bottom out on a core set of "primitive" constructs.

The primitive types are in the `mu` namespace and include:

* `mu/container`: A Docker container wrapped in Mu metadata.
* `mu/gateway`: An API gateway and/or load balancer, multiplexing requests onto multiple target Services.
* `mu/func`: A single Function ordinarily used for serverless/stateless scenarios.
* `mu/event`: An Event that may be used to Trigger the execution of another Service (commonly a Function).
* `mu/volume`: A volume stores data that can be mounted by another Service.
* `mu/autoscaler`: A Service that automatically multi-instances and scales some other target Service based on policy.
* `mu/extension`: A logical Service that extends Mu by hooking into events, like Stack provisioning, and taking action.

TODO(joe): link to exhaustive details on each of these.
TODO(joe): consider a `mu/job` (e.g., ECS's RunTask); unclear on how this would differ from `mu/func`.
TODO(joe): consider a `mu/daemon` type, similar to Kube's DaemonSet abstraction.

Although these may look like "magic", each primitive Stack simply leverages an open extensibility API in the platform.
Most interesting tasks may be achieved by composing existing Stacks, however, this extensibility API may be used to
define new, custom primitive Stacks for even richer functionality.  TODO(joe): more on this.

Finally, note that a companion namespace, `mu/x` also exists, that offers more cloud-neutral platform abstractions.

### Dependencies

We just saw that Service types can refer to other Stacks.  That is done with a so-called StackRef, which is simply a
name that contains multiple parts:

* An optional protocol (e.g., `https://`).
* An optional base URL (e.g., `hub.mu.com/`, `github.com/`, etc).
* A required namespace and/or name part (e.g., `acmecorp/worker`, `aws/s3/bucket`, etc).
* An optional `@` followed by version number (e.g., `@^1.0.6`, `@6f99088`, `@latest`, etc).

If protocol and base URL are absent, Mu will default to `https://hub.mu.com/`.  If the version is omitted, Mu will
default to `latest`, which just means "tip"; in other words, the most recent version is used at compile-time.

For Workspaces containing multiple Stacks, it can be advantageous to omit version information from your Stacks, and
instead place them into your `workspace.yaml` file's `dependencies` section.  For example:

    dependencies:
        aws/s3/bucket: ^1.0.6

This helps to manage version numbers centrally which can be especially convenient when upgrading.  Any StackRefs missing
version information will consult this workspace at compile-time.  You may even pin an entire namespace this way:

    dependencies:
        aws/...: ^1.0.6

Note that the compiled `Mu.yaml` will always contain pinned versions, so that it stands on its own.

Please refer to [Mu Dependencies](deps.md) for more details on dependencies and how they are resolved.

### Visibility

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

### Service Properties

We have already seen plenty of property setters.  But supplying capabilities -- or references to other Services --
warrants a special mention.

First, imagine we are creating a Stack that asks for a `service`; its `properties` might look like:

    name: acmecorp/factory
    properties:
        worker:
            type: service

Now let's create a Stack that instantiates `acmecorp/factory`, providing a reference to its own worker Service:

    name: my/factory
    services:
        private:
            myworker:
                type: mu/container
                build: .
                port: 80
            acmecorp/factory:
                worker: myworker

Notice that we have set `acmecorp/factory`'s `worker` factory to `myworker`.  This is a reference to our very own
`myworker` Service, instantiated in the section just prior.  At runtime, this simply expands to a URL referring to our
worker's public endpoint listening on port 80.

If multiple possible ports are available, an error will occur, and you will need to pick one explicitly.  For example:

            myworker:
                type: mu/container
                build: .
                ports: [ 80, 8080 ]

In this example, we still want to bind to port 80, however the system has no idea which to choose.  So we must say:

            acmecorp/factory:
                worker: myworker:80

A similar problem happens if we are passing another Service as the argument.  For example, let's say that we are using a
3rd party worker Service, rather than our own.  For instance:

            jazzcorp/jazzworker:
            acmecorp/factory:
                worker: jazzworker

This will of course work just fine, provided `jazzworker` has a public Service enpoint.  For example:

    name: jazzcorp/jazzworker
    services:
        public:
            api:
                port: 80
                ..

Much like how the system picked port 80 when there was only one in the raw container example, the system knows how to
pick the right public endpoint when there is just one (in this case, `api` on port 80).  However, just as the system
needed a hint when there were multiple possible ports, it may need one here too, such as in this example:

    name: jazzcorp/jazzworker
    services:
        public:
            api:
                port: 80
                ..
            admin:
                port: 8080
                ..

Now `jazzworker` has two public endpoints: `api` on port 80 and `admin` on port 8080.  We must select one:

            jazzcorp/jazzworker:
            acmecorp/factory:
                worker: jazzworker:api

Because this is a weakly typed example, using the base `service` type, plus a container, the amount of typechecking
performed is quite minimal.  In fact, just about the only thing it does is build a DAG of dependencies, ensure they are
cycle-free, and select the port.  Strongly typed examples, like the `ex/kvstore` one mentioned earlier, work similarly,
except that there is extra compile-time validation and more rugged selection of endpoints.

## Nested Stacks

Another feature that comes in handy sometimes is the ability to create nested Stacks:

    stacks:

Each nested Stack is very much like the Stack defined by any given Mufile, except that it is scoped, much like a
nested/inner class in object-oriented languages.  Doing this lets you subclass and/or multi-instance a single Stack as
multiple Services inside of the same Mufile.  For example, consider a container that will be multi-instanced:

    stacks:
        private:
            common:
                type: mu/container
                image: acmecorp/great
                env:
                    NAME: {{.meta.name}}-cluster
                    DATA: false
                    MASTER: false
                    HTTP: false

Now that we've defined `common`, we can go ahead and create it, without needing to expose the Stack to clients:

    services:
        private:
            data:
                type: common
                env:
                    DATA: true
        public:
            master:
                type: common
                env:
                    MASTER: true
            worker:
                type: common
                env:
                    HTTP: true

All of these three Services -- one private and two public -- leverage the same `acmecorp/great` container image,
and each one defines the same four set of environment variables.  Each instance, however, overrides a different
environment variable default value, to differentiate the roles as per the container's semantics.

Different scenarios call for subclassing versus composition, and the Mu system supports both in a first class way.

TODO(joe): we need to decide whether you can export public Stacks for public consumption.  At this point, my stance is
    that you must create an entirely different Stack to do that.  This keeps things simple for the time being.

## Target-Specific Metadata

Although for the most part, metadata strives to be cloud provider-agnostic, there are two ways in which it may not be.
First, some Stack types are available only on a particular cloud, like `aws/s3/bucket` (and any that transitively
reference this).  Attempting to cross-deploy Stacks referencing such things will fail at compile-time, for obvious
reasons.  Second, some metadata can be cloud provider-specific.  For example, even if we are creating a Service that is
logically independent from any given cloud, like a Load Balancer, we may wish to provider cloud-specific settings.
Those appear in a special metadata section and are marked in such a way that erroneous deployments fail at compile-time.

More details on target-specific Stacks and metadata settings are provided below in the relevant sections.

