# Mu Graph Language (MuGL)

In several cases, Mu creates and operates on object graphs.  Sometimes these object graphs are very general purpose, and
in other times, they are limited to subsets (such as resource-only DAGs).  These graphs are produced when evaluating
a MuPackage, when determining the resource graph that represents a deployment activity, and so on.  Anytime such a
graph must be persisted, Mu serializes using the Mu Graph Language (MuGL) format.  This document specifies MuGL.

## Overall Structure

The overall structure of the MuGL file format is straightforward; it consists of a linear list of objects, keyed by a
so-called *moniker*, which is a unique identifier within a single MuGL.  Each object contains a simple key/value bag of
properties, possibly deeply nested for complex structures.  These objects may reference each other using their monikers.

For example, this directed graph

       B
      ^ \
     /   v
    A     D -> E
     \   ^
      v /
       C

might be serialized as the following MuGL:

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

Any other fields are legal as peers to `vertices`, as is common with snapshots (e.g., to track the source MuPackage
and arguments).  The schema for vertices is similarly open-ended, except that `#ref` objects resolve to their
corresponding object counterparts upon deserialization into a runtime graph representation.

The `#ref` name is chosen to reduce the likelihood of conflicts with real property names; a MuGL file can override
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

Although MuGL is general purpose, it is used for one very specific area of the Mu system: *resource snapshots*.  Each
snapshot captures a complete end-to-end view of an environment's resources and their state.  These snapshots are used to
version infrastructure, to compare existing infrastructure to a set of changes, and ultimately, to deploy changes.

A snapshot's schema is identical to that shown above for general MuGL graphs, with these caveats:

* The source MuPackage and arguments, if any, are encoded in the MuGL's header section.

* All snapshot graphs are DAGs.  As such, the objects are in topologically-sorted order.

* Every object is a resource; data objects are serialized as regular JSON (and hence must be acyclic).

* All resource objects have a fixed schema.

* All resource monikers are "stable" (see below).

Each resource has a type token (in [the usual Mu sense](mupack.md)), an optional ID assigned by its provider, an
optional list of moniker aliases, and a bag of properties which, themselves, are just JSON objects with optional edges
inside.  Edges inside properties connect one resource to another; because snapshots are DAGs, all dependency resource
definitions will lexically precede the dependent resource within the MuGL file, ensuring single pass deserializability.

For example, imagine a resource snapshot involving a VPC, Subnet, SecurityGroup, and EC2 Instance:

       Subnet -> VPC
        ^         ^
       /          |
    Instance      |
       \          |
        v         |
       SecurityGroup

Assuming it was created from a `my/cluster` MuPackage, we might expect to find the following MuGL snapshot file:

    {
        "package": "my/cluster:*",
        "vertices": {
            "VPC": {
                "id": "vpc-30629859",
                "type": "aws:ec2/vpc:VPC",
                "properties": {
                    "cidrBlock": "172.31.0.0/16"
                }
            },
            "Subnet": {
                "id": "subnet-925087fb",
                "type": "aws:ec2/subnet:Subnet",
                "properties": {
                    "cidrBlock": "172.31.0.0/16",
                    "vpcId": { "#ref": "VPC" }
                }
            },
            "SecurityGroup": {
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
                    "vpc": { "#ref": "VPC" }
                }
            },
            "Instance": {
                "id": "i-0cd6974f17a414343",
                "type": "aws:ec2/instance:Instance",
                "properties": {
                    "imageId": "ami-f6035893",
                    "instanceType": "t2.micro",
                    "securityGroupIds": [
                        { "#ref": "SecurityGroup" }
                    ],
                    "subnetId": { "#ref": "Subnet" }
                }
            }
        }
    }

### Resource Monikers

A goal of snapshots is that, in addition to being unique, they are diffable and that resources in one graph may be
easily compared to like-resources in another graph.  Therefore, we desire some amount of stability to the monikers
chosen for resource objects.  The simple names like `VPC`, `Subnet`, and so on, from above, clearly won't do.

At the moment, these monikers are automatically generated by the system.  This is opposed to an alternative design in
which end-users specify them explicitly.  Manually assigning monikers would be problematic because we want every unique
environment, and indeed multiple deployments within the same environment, to have unique, yet diffable, names.

The algorithm for generating monikers is likely to evolve over time as we gain experience with them.  For now, they
encode the path from root to resource vertex within the original MuGL graph from which the resources were extracted.

It is possible there are multiple paths to the same resource, in which case, the shortest one is chosen as the primary
moniker; if all monikers are of equal length, the first lexicographically ordered one is chosen.  In any case, the
aliase monikers are available on the resource definition in case it helps resolve comparison ambiguities.

