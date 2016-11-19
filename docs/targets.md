# Mu Compilation Targets

This document describes how Mu metadata is compiled and deployed to various cloud targets.  Please refer to [the
companion metadata specification](metadata.md) to understand the source input in more detail.

There are two primary dimensions to any given target:

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

TODO(joe): describe the "local" cases, e.g. none(?), Docker, VirtualBox, HyperV, etc.

In all cases, the native metadata formats for the IaaS and CaaS provider in question is supported; for example, ECS on
AWS will leverage CloudFormation as the target metadata.  In certain cases, we also support Terraform outputs.

Refer to [marapongo/mu#2](https://github.com/marapongo/mu/issues/2) for an up-to-date prioritization of platforms.

## Clusters

A Stack is deployed to a Cluster.  Any given Cluster is a fixed combination of IaaS and CaaS provider.  Developers may
choose to manage Clusters and multiplex many Stacks onto any given Cluster, or they may choose to simply deploy a
Cluster per Stack.  The latter is of course easier, but may potentially incur more waste than the former.  Furthermore,
it will likely take more time to provision and modify entire Clusters than just the Stacks running within them.

Because creating and managing Clusters is a discrete step, the translation process will articulate them independently.
The tools make both the complex and simple workflows possible.

## Commonalities Among Targets

There are some common principles applied, no matter the target, which are worth calling out:

* DNS is the primary means of service discovery.
* TODO(joe): more...

## IaaS Targets

This section describes the translation for various IaaS targets.  Recall that deploying to an IaaS *without* any CaaS is
a supported scenario, so each of these descriptions is "self-contained."  In the case that a CaaS is utilized, that
process -- described below -- can override certain decisions made in the IaaS translation process.  For instance, rather
than leveraging a VM per Docker Container, the CaaS translation will choose to target an orchestration layer.

### Amazon Web Services (AWS)

The output of a transformation is one or more AWS CloudFormation templates.

#### Clusters

Each Cluster is given a standard set of resources.  If multiple Stacks are deployed into a shared Cluster, then those
Stacks will share all of these resources.  Otherwise, each Stack is given a dedicated set of them just for itself.

TODO(joe): compare with Convox Racks: https://convox.com/docs/rack.

##### Configuration

By default, all machines are placed into the XXX region and are given a size of YYY.  The choice of region may be
specified at provisioning time (TODO(joe): how), and the size may be changed as a Cluster-wide default (TODO(joe): how),
or on an individual Node basis (TODO(joe): how).

TODO(joe): multi-region.

TODO(joe): high availability.

TODO(joe): see http://kubernetes.io/docs/getting-started-guides/aws/ for reasonable defaults.

TODO(joe): see Empire for inspiration: https://s3.amazonaws.com/empirepaas/cloudformation.json, especially IAM, etc.

All Nodes in the Cluster are configured uniformly:

1. DNS for service discovery.
2. Docker volume driver for EBS-based persistence (TODO: how does this interact with Mu volumes).

TODO(joe): describe whether this is done thanks to an AMI, post-install script, or something else.

TODO(joe): CloudWatch.

TODO(joe): CloudTrail.

##### Identity, Access Management, and Keys

The AWS translation for security constructs follows the [AWS best practices for IAM and key management](
http://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html).  There is a fairly direct mapping between Mu
Users, Roles, and Groups, and the IAM equivalents with the same names.

AWS does not support Group nesting or inheritance, however.  Mu handles this by "template expansion"; that is, by
copying any parent Group metadata from parent to all of its ancestors.

TODO(joe): keys.

TODO(joe): auth tokens.

##### Networking

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

##### Discovery and Cluster State

Next, each Cluster gets a key/value store.  By default, this is Hashicorp Consul.  This is used to manage Cluster
configuration, in addition to a discovery service should a true CaaS orchestration platform be used (i.e., not VMs).

TODO(joe): it's unfortunate that we need to do this.  It's a "cliff" akin to setting up a Kube cluster.

TODO(joe): ideally we would use an AWS native key/value/discovery service (or our own, leveraging e.g. DynamoDB).

TODO(joe): this should be pluggable.

TODO(joe): figure out how to handle persistence.

TODO(joe): private container registries.

TODO(joe): encrypted secret storage (a la Vault).

#### Stacks/Services

Each Mu Stack compiles into a [CloudFormation Stack](
http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/stacks.html), leveraging a 1:1 mapping.  The only
exceptions to this rule are resource types that map directly to a CloudFormation resource name, backed either by a
standard AWS resource -- such as `AWS::S3::Bucket` -- or a custom one -- such as one of the Mu primitive types.

We also leverage [cross-Stack references](
http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/walkthrough-crossstackref.html) to wire up references.

This approach means that you can still leverage all of the same CloudFormation tooling on AWS should you need to.  For
example, your IT team might have existing policies and practices in place that can be kept.  Managing Stacks through the
Mu tools, however, is still ideal, as it is easier to keep your code, metadata, and live site in synch.

TODO(joe): we need a strategy for dealing with AWS limits exhaustion; e.g.
    http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cloudformation-limits.html.

TODO(joe): should we support "importing" or "referencing" other CloudFormation Stacks, not in the Mu system?

The most interesting question is how Mu projects the primitive concepts in the system into CloudFormation metadata.  For
most Stacks, this is just "composition" that falls out from name substitution, etc.; however, the primitive concepts
introduce "abstraction" and therefore manifest as groupings of physical constructs.  Let us take them in order.

TODO(joe): I'm still unsure whether each of these should be a custom CloudFormation resource type (e.g.,
    `Mu::Container`, `Mu::Gateway`, etc).  This could make it a bit nicer to view in the AWS tools because you'd see
    our logical constructs rather than the deconstructed form.  It's a little less nice, however, in that it's more
    complex implementation-wise, requiring dynamic Lambda actions that I'd prefer to be static compilation actions.

`mu/container` maps to a single `AWS::EC2::Instance`.  However, by default, it runs a custom AMI that uses our daemon
for container management, including configuration, image pulling policies, and more.  (Note that, later on, we will see
that running a CaaS layer completely changes the shape of this particular primitive.)

`mu/gateway` maps to a `AWS::ElasticLoadBalancing::LoadBalancer` (specifically, an [Application Load Balancer](
https://aws.amazon.com/elasticloadbalancing/applicationloadbalancer/)).  Numerous policies are automatically applied
to target the Services wired up to the Gateway, including routine rules and tables.  In the event that a Stack is
publically exported from the Cluster, this may also entail modifications of the overall Cluster's Ingress/Egress rules.

TODO: `mu/func` and `mu/event` are more, umm, difficult.

`mu/volume` is an abstract Stack type and so has no footprint per se.  However, implementations of this type exist that
do have a footprint.  For example, `aws/ebs/volume` derives from `mu/volume`, enabling easy EBS-based container
persistence.  Please refer to the section below on native AWS Stacks to understand how this particular one works.

`mu/autoscaler` generally maps to an `AWS::AutoScaling::AutoScalingGroup`, however, like the Gateway's mapping to the
ELB, its mapping to the AWS scaling group entails a lot of automatic policy to properly scale attached Services.

Finally, `mu/extension` is special, and doesn't require a specific mapping in AWS.  The extension providers themselves,
like `aws/cf/template`, will possibly generate domain-specific mappings of their own, however.

TODO(joe): perhaps we should have an `aws/cf/customresource` extension type for custom CloudFormation types.

#### AWS-Specific Metadata

#### AWS-Specific Stacks

As we saw above, AWS services are available as Stacks.  Let us now look at how they are expressed in Mu metadata and,
more interestingly, how they are transformed to underlying resource concepts.  It's important to remember that these
aren't "higher level" abstractions in any sense of the word; instead, they map directly onto AWS resources.  (O course,
other higher level abstractions may compose these platform primitives into more interesting services.)

A simplified S3 bucket Stack, for example, looks like this:

    name: bucket
    properties:
        accessControl: string
        bucketName: string
        corsConfiguration: aws/schema/corsConfiguration
        lifecycleConfiguration: aws/schema/lifecycleConfiguration
        loggingConfiguration: aws/schema/loggingConfiguration
        notificationConfiguration: aws/schema/notificationConfiguration
        replicationConfiguration: aws/schema/replicationConfiguration
        tags: [ aws/schema/resourceTag ]
        versioningConfiguration: aws/schema/versioningConfiguration
        websiteConfiguration: aws/schema/websiteConfigurationType
    services:
        public:
            mu/extension:
                provider: aws/cf/template
                template: |
                    {
                        "Type": "AWS::S3::Bucket",
                        "Properties": {
                            "AccessControl": {{json .props.accessControl}},
                            "BucketName": {{json .props.bucketName}},
                            "CorsConfiguration": {{json .props.corsConfiguration}},
                            "LifecycleConfiguration": {{json .props.lifecycleConfiguration}},
                            "NotificationConfiguration": {{json .props.notificationConfiguration}},
                            "ReplicationConfiguration": {{json .props.replicationConfiguration}},
                            "Tags": {{json .props.tags}},
                            "VersioningConfiguration": {{json .props.versioningConfiguration}},
                            "WebsiteConfiguration": {{json .props.websiteConfiguration}}
                        }
                    }

The key primitive at play here is `mu/extension`.   This passes off lifecycle events to a provider, in this case
`aws/cf/template`, along with some metadata, in this case a simple wrapper around the [AWS CloudFormation S3 Bucket
specification format](
http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-s3-bucket.html).  The provider generates
metadata and knows how to interact with AWS services required for provisioning, updating, and destroying resources.

TODO(joe): we need to specify how extensions work somewhere.

Mu offers all of the AWS resource type Stacks out-of-the-box, so that 3rd parties can consume them easily.  For example,
to create a bucket, we simply refer to the predefined `aws/s3/bucket` Stack.  Please see [the AWS documentation](
http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-template-resource-type-ref.html) for an exhaustive
list of available services.

TODO(joe): should we be collapsing "single resource" stacks?  Seems superfluous and wasteful otherwise.

### Google Cloud Platform (GCP)

### Microsoft Azure

### VMWare

## CaaS Targets

All of the IaaS targets above described the default behavior when deploying containers, which is to map each container
to a dedicated VM instance.  This is secure, robust, and easy to reason about, but can be wasteful.  A CaaS framework
like Docker Swarm, Kubernetes, Mesos, or one of the native cloud provider container services, can bring about
efficiencies by multiplexing many containers onto a smaller shared pool of physical resources.  This section describes
the incremental differences brought about when targeting such a framework.

### Docker Swarm

TODO(joe): figure out how Docker InfraKit does or does not relate to all of this (maybe even beyond Swarm target).

### Kubernetes

### Mesos

### AWS EC2 Container Service (ECS)

Targeting the [ECS](http://docs.aws.amazon.com/AmazonECS/latest/developerguide/) CaaS lets AWS's native container
service manage scheduling of containers on EC2 VMs.  It is only legal when using the AWS IaaS provider.

First and foremost, every Cluster containing at least one `mu/container` in its transitive closure of Stacks gets an
associated [ECS cluster](http://docs.aws.amazon.com/AmazonECS/latest/developerguide/ECS_GetStarted.html).

A reasonable default number of instances, of a predefined type, are chosen, but you may override them (TODO(joe): how).
All of the AWS-wide settings, such as IAM, credentials, and region, are inherited from the base AWS IaaS configuration.

The next difference is that, rather than provisioning entire VMs per `mu/container`, each one maps to an [ECS service](
http://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_services.html).

TODO(joe): describe the auto-scaling differences.  In ECS, service auto-scaling is *not* the same as ordinary EC2
    auto-scaling.  (See [this](http://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-auto-scaling.html).)
    This could cause some challenges around the composition of `mu/autoscaler`, particularly with encapsulation.

TODO(joe): if we do end up supporting a `mu/job` type, we would presumably map it to ECS's CreateTask construct.

### Google Container Engine (GKE)

### Azure Container Service (ACS)

## Terraform

TODO(joe): describe what Terraform may be used to target and how it works.

## Redeploying Cluster and Stack Deltas

TODO(joe): describe how we perform delta checking in `$ mu apply` and how that impacts the various target generations.

TODO(joe): look into how Convox does this https://convox.com/guide/reloading/, and others.

