# Mu Metadata

This document contains a formal description of Mu's metadata, in addition to the translation process for all current
targets.

## Specification

This section includes the specification for Mu's metadata plus its core platform primitives.

### Platform

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

### Metadata

The essential artifact a developer uses to create Stacks and Services is something we call a Mufile.  It is
conventionally named `Mu.yaml`, usually checked into the Workspace, and each single file describes a Stack.  Note that
Stacks may internally contain other Stacks, however this is purely an implementation detail of how the Stack works.

#### Package Managament

Each Mufile begins with some standard "package manager"-like metadata, like name, version, description, and so on.  As
with most package managers, most of these elements are optional:

    name: elk
    version: 1.0.1
    description: A fully functioning ELK stack (Elasticsearch, Logstash, Kibana).
    author: Joe Smith <joesmith@elk.com>
    website: https://github.com/joesmith/elk

TODO(joe): finish this section.

#### Services

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

#### Nested Stacks

Another feature that comes in handy sometimes is the ability to create nested Stacks:

    stacks:

Each nested Stack is very much like the Stack defined by any given Mufile, except that it is scoped, much like a
nested/inner class in object-oriented languages.  There are two primary reasons you might do this:

1. Multi-instancing that Stack, to create multiple Services.
2. Conveniently leveraging that Stack as the "type" for one or more other Services in the file.

TODO(joe): examples of the above.

TODO(joe): we need to decide whether you can export public Stacks for public consumption.  At this point, my stance is
that you must create an entirely different Stack to do that.  This keeps things simple for the time being.

### An Illustrative Example

Before breaking down the implementation of these concepts, let's first look at an illustrative example.

TODO(joe): a more comprehensive example (like Vote50) that illustrates more of these concepts working in harmony.

## Targets

This section contains a precise description of the mapping process from Mu metadata to various cloud targets.  Note
that there are two dimensions to this process:

* The first dimension is the system used for container orchestration, or what we will call, Containers-as-a-Service
  (CaaS).  Examples of this include AWS ECS, Docker Swarm, Kubernetes, and even plain old VM-based scheduling.

* The second dimension is the system used for hosting the VMs fueling the CaaS, which we will call
  Infrastructure-as-a-Service (IaaS).  Examples of this include AWS, Google Cloud Platform (GCP), Azure, and even VM
  fabrics for on-premise installations, like VMWare VSphere.  Note that often IaaS goes beyond simply having VMs as
  resources and can include hosted offerings such as blob storage, load balancers, domain name configurations, etc.

Not all combinations of CaaS and IaaS fall out naturally, although it is a goal of the system to target them
orthogonally such that the incremental cost of creating new pairings is as low as possible (minimizing combinatorics).
Some combinations are also clearly nonsense, such as using AWS ECS as the CaaS and GCP as the IaaS.

For reference purposes, here is a compatibility matrix for what we envision supporting.  Each column with an `X` is
described in this document already; each column with an `-` is planned, but not yet described; and `n/a` is nonsense:

|               | AWS       | GCP       | Azure     | VMWare    |
+---------------+-----------+-----------+-----------+-----------|
| VM            | X         | -         | -         | -         |
| Docker Swarm  | -         | -         | -         | -         |
| Kubernetes    | -         | -         | -         | -         |
| Mesos         | -         | -         | -         | -         |
| ECS           | X         | n/a       | n/a       | n/a       |
| GKE           | n/a       | -         | n/a       | n/a       |
| ACS           | n/a       | n/a       | -         | n/a       |

In all cases, the native metadata formats for the CaaS and IaaS provider in question is supported; for example, ECS on
AWS will leverage CloudFormation as the target metadata.  In certain cases, we also support Terraform outputs.

### CaaS Targets

#### VM

#### Docker Swarm

#### Kubernetes

#### AWS EC2 Container Service (ECS)

#### Google Container Engine (GKE)

#### Azure Container Service (ACS)

### IaaS Targets

#### Amazon Web Services (AWS)

#### Google Cloud Platform (GCP)

#### Microsoft Azure

#### VMWare

### Specific Combinations

### Terraform

TODO(joe): describe what Terraform may be used to target and how it works.

