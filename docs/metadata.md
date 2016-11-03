# Mu Metadata

This document contains a formal description of Mu's metadata, in addition to the translation process for all current
targets.

## Specification

This section includes the specification for Mu's metadata plus its core platform primitives.

The essential artifact a developer uses to create Stacks and Services is something we call a Mufile.  It is
conventionally named `Mu.yaml`, usually checked into the Workspace, and each single file describes a Stack.  (Note that
Stacks may internally contain other Stacks, however this is purely an implementation detail of how the Stack works.)

TODO(joe): declarative specification format for Clusters.

Although all examples are in YAML, it is perfectly valid to use JSON instead if that is more desirable.

Mu preprocesses all metadata files to substitute context values not known until runtime, such as configuration,
arguments, and so on.  The [Go template syntax](https://golang.org/pkg/text/template/) is used for this.  Please refer
to the API documentation for the context object (TODO(joe): do this) for details on what information is available.

### Package Managament

Each Mufile begins with some standard "package manager"-like metadata, like name, version, description, and so on.  As
with most package managers, most of these elements are optional:

    name: elk
    version: 1.0.1
    description: A fully functioning ELK stack (Elasticsearch, Logstash, Kibana).
    author: Joe Smith <joesmith@elk.com>
    website: https://github.com/joesmith/elk

TODO(joe): finish this section.

### Subclassing

A Stack may subclass any other Stack, specializing aspects of it as appropriate.  This facilitates reuse.  For instance,
perhaps my company wishes to enforce that certain best practices and standards are adhered to, for all Stacks.  Or
imagine someone in the community has published a best-in-breed Node.js application blueprint, leveraging Express,
MongoDB, and the ELK stack, and I merely want to plug in my own application logic and leverage the overall Stack.

To do this, reference another Stack's fully qualified name in the `base` property:

    base: some/other/stack

From there, I can use all of the other metadata in this section, however we will have inherieted anything in the base.

TODO(joe): what about mixins?
TODO(joe): get more specific about what can be overridden.  Furthermore, what about "deletes"?

### APIs

Every Stack may choose to export one or more APIs.  These APIs can be standard "unconstrained" network interfaces, such
as "HTTP over port 80", or can take on a more structured form, like leveraging OpenAPI to declare a protocol interface.
The benefits of declaring the full interfaces are that the RPC semantics are known to the system, facilitating advanced
management capabilities such as diagnostics, monitoring, fuzzing, and self-documentation, in addition to RPC code-
generation.  This also adds a sort of "strong typing" to the connections between Services.

TODO(joe): articulate this section further; e.g., the metadata format, precise advantages, etc.

### Parameters

Each Stack can declare a set of parameters that callers must supply, in the `parameters` section:

    parameters:

Each parameter has the following properties:

* `name`: A name unique amongst all parameters.
* `type`: A parameter type, restricting the legal values.
* `default`: A default value to be supplied if missing from the caller.
* `optional`: If `true`, this parameter is optional.

The set of types a parameter may take on are "JSON-like".  This includes simple primitives:

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

    name: tag
    type:
        id: number
        name: string
        value: object

The most interesting capability here is the ability to request a "capability", or reference to another Service.  This
provides a strongly typed and more formal way of expressing Service dependencies, in a way that the system can
understand and leverage in its management of the system (like ensuring Services are created in the right order).  It
also eliminates some of the fragility of weakly typed and dynamic approaches, which can be prone to race conditions.

The most basic form is to use the special type `service`:

    type: service

This is helpful, as it exposes a dependency to the system, but it isn't perfect.  The shape of the dependency is still
opaque to the system.  A step further is to express that a specific port is utilized:

    type: service:80

This declaration says that we require a Service with an exposed port 80.  This strong typing flows through the system,
permitting liveness detection, and even compile-time type checking that the supplied Service argument actually does
expose something on port 80 -- in the event that this ever changes, we will find out upon recompilation.

Even better still is to declare that we depend on a specific kind of Service, by specifying the fully qualified name of
a Stack.  In such a case, the system ensures an instance of this Stack type, or subclass, is provided:

    type: examples/keyValueStore

This hypothetical Stack defines an API that can be used as a key-value store.  Presumably we would find subclasses of it
for etcd, Consul, Zookeeper, and others, which a caller is free to choose from at instantiation time.

Another example leverages the primitive `mu/volume` type to require a Service which can be mounted as a volume:

    type: mu/volume

Finally, note that anywhere inside of this Mufile, we may access the arguments supplied at Stack instantiation time
using the Go template syntax mentioned earlier.  For example, `{{.args.tag.name}}`.

### Configuration

### Services

After that comes the section that describes what Services make up this Stack:

    services:

In this section is zero-to-many Services that are co-created with one another.  Each Service has:

* A name, both for dynamic and static use.
* A type, which is just the name of a Stack to instantiate.
* A visibility governing whether consumers of this Stack have access to it or not.
* One or more named arguments, mapping to the Stack's constructor parameters.

Although these Services are co-created, they may reference one another.  The references between each other forms a DAG
and the system topologically sorts that DAG in order to determine the order in which to create and destroy Services.
Notably there may be no cycles.  By default, the system understands liveness and health (TODO(joe): how); as a result,
the developer need not explicitly worry about races, liveness, or retries during Service creation.

#### Names

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

#### Types

Each Service has a type, which is merely the name of another Stack.  Most of the time this is the FQN, although for
references to other Stacks defined within the same Mufile (more on that later), this can just be a simple name.  During
instantiation of that Service, a fresh instance of that Stack is created and bound to in place of this Service name.

Although there are obviously many opportunities for ecosystems of user-defined Stacks, and indeed a rich library offered
by the Mu platofrm itself, we eventually bottom out on a core set of "primitive" constructs.

The primitive types available include:

* `mu/container`: A Docker container wrapped in Mu metadata.
* `mu/gateway`: An API gateway and/or load balancer, multiplexing requests onto multiple target Services.
* `mu/func`: A single Function ordinarily used for serverless/stateless scenarios.
* `mu/event`: An Event that may be used to Trigger the execution of another Service (commonly a Function).
* `mu/volume`: A volume stores data that can be mounted by another Service.
* `mu/autoscaler`: A Service that automatically multi-instances and scales some other target Service based on policy.
* `mu/hook`: A logical Service that has no concrete runtime manifestation other than running pre- or post-logic.

TODO(joe): link to exhaustive details on each of these.

Although these may look like "magic", each primitive Stack simply leverages an open extensibility API in the platform.
Most interesting tasks may be achieved by composing existing Stacks, however, this extensibility API may be used to
define new, custom primitive Stacks for even richer functionality.  TODO(joe): more on this.

#### Visibility

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

#### Constructor Arguments

### Nested Stacks

Another feature that comes in handy sometimes is the ability to create nested Stacks:

    stacks:

Each nested Stack is very much like the Stack defined by any given Mufile, except that it is scoped, much like a
nested/inner class in object-oriented languages.  Doing this lets you subclass and/or multi-instance a single Stack as
multiple Services inside of the same Mufile.  For example, consider a container that will be multi-instanced:

    stacks:
        private:
            - common:
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
            - data:
                type: common
                env:
                    DATA: true
        public:
            - master:
                type: common
                env:
                    MASTER: true
            - worker:
                type: common
                env:
                    HTTP: true

All of these three Services -- one private and two public -- leverage the same `acmecorp/great` container image,
and each one defines the same four set of environment variables.  Each instance, however, overrides a different
environment variable default value, to differentiate the roles as per the container's semantics.

Different scenarios call for subclassing versus composition, and the Mu system supports both in a first class way.

TODO(joe): we need to decide whether you can export public Stacks for public consumption.  At this point, my stance is
that you must create an entirely different Stack to do that.  This keeps things simple for the time being.

### Target-Specific Metadata

Although for the most part, metadata strives to be cloud provider-agnostic, there are two ways in which it may not be.
First, some Stack types are available only on a particular cloud, like `aws/s3/bucket` (and any that transitively
reference this).  Attempting to cross-deploy Stacks referencing such things will fail at compile-time, for obvious
reasons.  Second, some metadata can be cloud provider-specific.  For example, even if we are creating a Service that is
logically independent from any given cloud, like a Load Balancer, we may wish to provider cloud-specific settings.
Those appear in a special metadata section and are marked in such a way that erroneous deployments fail at compile-time.

More details on target-specific Stacks and metadata settings are provided below in the relevant sections.

### An Illustrative Example

Before breaking down the implementation of these concepts, let's first look at an illustrative example.

TODO(joe): a more comprehensive example (like Vote50) that illustrates more of these concepts working in harmony.

## Targets

This section contains a precise description of the mapping process from Mu metadata to various cloud targets.  Note
that there are two dimensions to this process:

* The first dimension is the system used for hosting the cluster environment, which we will call
  Infrastructure-as-a-Service (IaaS).  Examples of this include AWS, Google Cloud Platform (GCP), Azure, and even VM
  fabrics for on-premise installations, like VMWare VSphere.  Note that often IaaS goes beyond simply having VMs as
  resources and can include hosted offerings such as blob storage, load balancers, domain name configurations, etc.

* The second dimension is the system used for container orchestration, or what we will call, Containers-as-a-Service
  (CaaS).  Examples of this include AWS ECS, Docker Swarm, and Kubernetes.  Note that the system can handle the
  siituation where there is no container orchestration framework available, in which case raw VMs are utilized.

Not all combinations of IaaS and CaaS fall out naturally, although it is a goal of the system to target them
orthogonally such that the incremental cost of creating new pairings is as low as possible (minimizing combinatorics).
Some combinations are also clearly nonsense, such as AWS as your IaaS and GKE as your CaaS.

For reference, here is a compatibility matrix.  Each cell with an `X` is described in this document already; each cell
with an `-` is planned, but not yet described; and blank entries are unsupported nonsense combinations:

|               | AWS       | GCP       | Azure     | VMWare    |
| ------------- | --------- | --------- | --------- | --------- |
| none (VMs)    | X         | -         | -         | -         |
| Docker Swarm  | -         | -         | -         | -         |
| Kubernetes    | -         | -         | -         | -         |
| Mesos         | -         | -         | -         | -         |
| ECS           | X         |           |           |           |
| GKE           |           | -         |           |           |
| ACS           |           |           | -         |           |

In all cases, the native metadata formats for the IaaS and CaaS provider in question is supported; for example, ECS on
AWS will leverage CloudFormation as the target metadata.  In certain cases, we also support Terraform outputs.

Refer to [marapongo/mu#2](https://github.com/marapongo/mu/issues/2) for an up-to-date prioritization of platforms.

### Clusters

A Stack is deployed to a Cluster.  Any given Cluster is a fixed combination of IaaS and CaaS provider.  Developers may
choose to manage Clusters and multiplex many Stacks onto any given Cluster, or they may choose to simply deploy a
Cluster per Stack.  The latter is of course easier, but may potentially incur more waste than the former.  Furthermore,
it will likely take more time to provision and modify entire Clusters than just the Stacks running within them.

Because creating and managing Clusters is a discrete step, the translation process will articulate them independently.
The tools make both the complex and simple workflows possible.

### Commonalities

There are some common principles applied, no matter the target, which are worth calling out:

* DNS is the primary means of service discovery.
* TODO(joe): more...

### IaaS Targets

This section describes the translation for various IaaS targets.  Recall that deploying to an IaaS *without* any CaaS is
a supported scenario, so each of these descriptions is "self-contained."  In the case that a CaaS is utilized, that
process -- described below -- can override certain decisions made in the IaaS translation process.  For instance, rather
than leveraging a VM per Docker Container, the CaaS translation will choose to target an orchestration layer.

#### Amazon Web Services (AWS)

The output of a transformation is one or more AWS CloudFormation templates.

##### Clusters

Each Cluster is given a standard set of resources.  If multiple Stacks are deployed into a shared Cluster, then those
Stacks will share all of these resources.  Otherwise, each Stack is given a dedicated set of them just for itself.

TODO(joe): IAM.
TODO(joe): keys.

By default, all machines are placed into the XXX region and are given a size of YYY.  The choice of region may be
specified at provisioning time (TODO(joe): how), and the size may be changed as a Cluster-wide default (TODO(joe): how),
or on an individual Node basis (TODO(joe): how).

TODO(joe): multi-region.
TODO(joe): high availability.
TODO(joe): see http://kubernetes.io/docs/getting-started-guides/aws/ for reasonable defaults.
TODO(joe): see Empire for inspiration: https://s3.amazonaws.com/empirepaas/cloudformation.json, especially IAM, etc.

Each Cluster gets a Virtual Private Cloud (VPC) for network isolation.  Along with this VPC comes the standard set of
sub-resources: a Subnet, Internet Gateway, and Route Table.  By default, Ingress and Egress ports are left closed.  As
Stacks are deployed, ports are managed automatically (although an administrator can lock them (TODO(joe): how)).

TODO(joe): open SSH by default?
TODO(joe): joining existing VPCs.
TODO(joe): how to override default settings.
TODO(joe): multiple Availability Zones (and a Subnet per AZ); required for ELB.
TODO(joe): HTTPS certs.
TODO(joe): describe how ports get opened or closed (e.g., top-level Stack exports).
TODO(joe): articulate how Route53 gets configured.
TODO(joe): articulate how ELBs do or do not get created for the cluster as a whole.

Next, each Cluster gets a key/value store.  By default, this is Hashicorp Consul.  This is used to manage Cluster
configuration, in addition to a discovery service should a true CaaS orchestration platform be used (i.e., not VMs).

TODO(joe): it's unfortunate that we need to do this.  It's a "cliff" akin to setting up a Kube cluster.
TODO(joe): ideally we would use an AWS native key/value/discovery service (or our own, leveraging e.g. DynamoDB).
TODO(joe): this should be pluggable.
TODO(joe): figure out how to handle persistence.

All Nodes in the Cluster are configured uniformly:

1. DNS for service discovery.
2. Docker volume driver for EBS-based persistence (TODO: how does this interact with Mu volumes).

TODO(joe): describe whether this is done thanks to an AMI, post-install script, or something else.

TODO(joe): CloudWatch.
TODO(joe): CloudTrail.
TODO(joe): private container registries.

##### Stacks/Services

##### AWS-Specific Stacks

TODO(joe): describe all the AWS-specific Stacks, e.g. `aws/sqs/queue`, `aws/s3/bucket`, `aws/dynamodb/table`, etc.

##### AWS-Specific Metadata

#### Google Cloud Platform (GCP)

#### Microsoft Azure

#### VMWare

### CaaS Targets

#### VM

#### Docker Swarm

#### Kubernetes

#### Mesos

#### AWS EC2 Container Service (ECS)

#### Google Container Engine (GKE)

#### Azure Container Service (ACS)

### Terraform

TODO(joe): describe what Terraform may be used to target and how it works.

