# Lumi Graph Language (LumiGL)

In several cases, Lumi creates and operates on object graphs.  Sometimes these object graphs are very general
purpose, and in other times, they are limited to subsets (such as resource-only DAGs).  These graphs are produced when
evaluating a package, when determining the resource graph that represents a deployment activity, etc.  Anytime such a
graph must be persisted, Lumi serializes using its own graph language (LumiGL).  This document specifies LumiGL.

## Overall Structure

The overall structure of the LumiGL file format is straightforward; it consists of a linear list of objects, each keyed
by an ID which is unique within the file.  Each object contains a simple key/value bag of properties, possibly deeply
nested for complex structures.  These objects may reference each other using their IDs.

For example, assuming that `#ref` is the means of a property referencing other vertices, this directed graph

       B
      ^ \
     /   v
    A     D -> E
     \   ^
      v /
       C

might be serialized as the following LumiGL:

    {
        "vertices": {
            "e": {
            },
            "d": {
                "children": [
                    { "#ref": "e" }
                ],
            },
            "c": {
                "children": [
                    { "#ref": "d" }
                ],
            },
            "b": {
                "children": [
                    { "#ref": "d" }
                ],
            },
            "a": {
                "children": [
                    { "#ref": "b" },
                    { "#ref": "c" }
                ],
            }
        }
    }

Any other fields are legal as peers to `vertices`, as is common with snapshots (e.g., to track the source LumiPack
and arguments).  The schema for vertices is similarly open-ended, except that `#ref` objects resolve to their
corresponding object counterparts upon deserialization into a runtime graph representation.

The `#ref` name is chosen to reduce the likelihood of conflicts with real property names; a LumiGL file can override
this choice with the special property `ref` in the front matter; for example, this uses `@@r`:

    {
        "ref": "@@r",
        "vertices": {
            ...,
            "a": {
                "children": [
                    { "@@r": "b" },
                    { "@@r": "c" }
                ],
            }
        }
    }

## Resource Snapshots

Although LumiGL is general purpose, it is used for one very specific area of the Lumi system: *resource snapshots*.
Each snapshot captures a complete end-to-end view of an environment's resources and their state.  These are used to
version infrastructure, to compare existing infrastructure to a set of changes, and ultimately, to deploy changes.

A snapshot's schema is identical to that shown above for general LumiGL graphs, with these caveats:

* The source LumiPack and arguments, if any, are encoded in the LumiGL's header.

* Every object is a resource; property values are serialized as flattened JSON values.

* As such, rather than "vertices", the section is labeled "resources".

* All resource objects have a fixed schema (see below).

* All resource IDs correspond to the auto-generated resource URNs (see [resources](resources.md)).

* All snapshot graphs are DAGs.  As such, resources are conventionally listed in topologically-sorted order.

In addition to its URN, which is also its key, each resource has a type token (in [the usual LumiIL sense](
packages.md)), an optional ID assigned by its provider, and a bag of properties which, themselves, are just JSON objects
with optional edges inside (encoded with `#ref`s).  Edges inside properties connect one resource to another; because
snapshots are DAGs, and in topologically-sorted order, all dependency resource definitions will lexically precede the
dependent resource within the LumiGL file, allowing for simple, single-pass deserialization.

For example, imagine a resource snapshot involving an AWS EC2 VPC, Subnet, SecurityGroup, and Instance:

       Subnet -> VPC
        ^         ^
       /          |
    Instance      |
       \          |
        v         |
       SecurityGroup

Assuming it was created from a `my/cluster` LumiPack, we might expect to find the following LumiGL snapshot file:

    {
        "package": "my/cluster#*",
        "resources": {
            "urn:lumi:prod::my/cluster:index::aws:ec2/vpc:VPC::cloud-vpc": {
                "id": "vpc-30629859",
                "type": "aws:ec2/vpc:VPC",
                "properties": {
                    "cidrBlock": "172.31.0.0/16"
                }
            },
            "urn:lumi:prod::my/cluster:index::aws:ec2/subnet:Subnet::cloud-subnet": {
                "id": "subnet-925087fb",
                "type": "aws:ec2/subnet:Subnet",
                "properties": {
                    "cidrBlock": "172.31.0.0/16",
                    "vpcId": { "#ref": "urn:lumi:prod::my/cluster:index::aws:ec2/vpc:VPC::cloud-vpc" }
                }
            },
            "urn:lumi:prod::my/cluster:index::aws:ec2/securityGroup:SecurityGroup::admin": {
                "id": "sg-151cd67c",
                "type": "aws:ec2/securityGroup:SecurityGroup",
                "properties": {
                    "name": "SSH",
                    "groupDescription": "Enable SSH access",
                    "securityGroupIngress": [
                        {
                            "cidrIp": "0.0.0.0",
                            "fromPort": 22,
                            "ipProtocol": "tcp",
                            "toPort": 22
                        }
                    ]
                    "vpcId": { "#ref": "urn:lumi:prod::my/cluster:index::aws:ec2/vpc:VPC::cloud-vpc" }
                }
            },
            "urn:lumi:prod::my/cluster:index::aws:ec2/instance:Instance::master": {
                "id": "i-0cd6974f17a414343",
                "type": "aws:ec2/instance:Instance",
                "properties": {
                    "imageId": "ami-f6035893",
                    "instanceType": "t2.micro",
                    "securityGroupIds": [
                        { "#ref": "urn:lumi:prod::my/cluster:index::aws:ec2/securityGroup:SecurityGroup::admin" }
                    ],
                    "subnetId": {
                        "#ref": "urn:lumi:prod::my/cluster:index::aws:ec2/subnet/Subnet::cloud-subnet"
                    }
                }
            }
        }
    }

A goal of snapshots is that, in addition to being unique, they are diffable and resources in one graph may be easily
compared to like-resources in another graph to produce a structured delta.  This ensures that snapshots are versionable
and, in fact, will version quite nicely in a source control management system like Git.

Due to this, plus the uniqueness requirement, clearly simple names -- like `VPC`, `Subnet`, etc. -- won't do.  That's
why the URNs in the example above are rather lengthy, and include so much of the associated resource object context.

Please refer to the [resources design note](resources.md) for additional details on the URN generation process.

TODO[pulumi/pulumi-fabric#30]: queryability (GraphQL?  RDF/SPARQL?  Neo4j/Cypher?  Gremlin?  Etc.)

TODO[pulumi/pulumi-fabric#109]: dynamic linking versus the default of static linking.

TODO[pulumi/pulumi-fabric#90]: specify how "holes" show up during planning ("<computed>").  E.g., control flow simulation.

TODO: describe what happens in the face of partial application failure.  Do graphs become tainted?

