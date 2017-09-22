# Lumi Resources

Lumi describes cloud resources and programs for purposes of provisioning, updating, and deleting.  From the LumiPack
and LumiIL documentation, it won't yet be clear how this happens.  The LumiPack and LumiIL formats are intentionally
general purpose and themselves contain no specific understanding of cloud resources.  This design note fills that gap.

## Overview

Each LumiPack contains program logic that encodes program logic and data.  This data comes in the form of the usual
programming language runtime constructs, namely primitive data (numbers, strings, etc) and objects.  For example:

    let bucket = new s3.Bucket("pictures", { deletionPolicy: "retain" });

The evaluation of an executable package, and the resulting runtime objects and data, drives the creation of a LumiGL
resource graph.  This is a directed acyclic graph (DAG) whose nodes represent resources and whose edges represent
resource dependencies.  From this graph, the Lumi toolchain can determine how to provision the desired resources.

The question is, how does it "provision" them?

## What is a Resource, Anyway

First thing's first.  For this to work, we must first identify what a "resource" is.

A resource in the programming model is simply identified by any class that derives from the `pulumi.Resource` base
class.  (The specific manifestation of this depends on your language SDK; examples below are in LumiJS.)  This class
is primarily a marker class in that it doesn't offer much on its own (except for a few advanced methods, see below).

    import * as lumi from "@lumi/lumi";
    class MyResouce extends lumi.Resource {
        constructor(name: string, ...) {
            super(name);
            // ...
        }
        // ...
    }

The only property that `Resource` demands is a "name".  This is a friendly name that is used to create resource URNs.

To create a LumiGL graph, Lumi evaluates the program.  As it does so, the Lumi runtime monitors object creation
and property assignments.  The runtime recognizes which of those objects are resources and records associated activity.

Each resource may have any number of properties.  Some properties may form dependencies on other resources, either by
storing a reference to another resource object, or by allocating one (directly or indirectly).  These dependencies are
ultimately used to form the LumiGL DAG.  In some rare circumstances, such as dynamic name-based dependencies -- a design
pattern not used by Lumi but by other cloud providers -- these dependencies may need to be registered explicitly.

Some properties may not have values until the resulting LumiGL graph is actually applied to a target environment; these
are called *provisioned properties* and will receive default values from the resource provider in some manner, possibly
by contacting the target cloud provider (e.g., default VM sizes, etc).  It is possible to use these properties, however,
we must tread with care.  A provisioned property's value cannot be known during planning.  Any conditional logic that
depends upon a provisioned property would mean an inability to plan, and so is rejected by default.

Most of the magic of a true resource is available in its *resource provider* plugin.  This provider implements create,
read, update, and delete (CRUD) operations for that resource type, in addition to some optional facilities, such as
logging and operational performance counters.  These plugins are registered and loaded dynamically (see more below).

## Resource Names, IDs, and URNs

Each resource has two unique IDs associated with it:

1. A provider-assigned ID that is generally opaque to Lumi.
2. A globally-unique Lumi-assigned
   [Uniform Resource Name (URN)](https://en.wikipedia.org/wiki/Uniform_Resource_Name).

As an example of the provider ID, AWS resources will have [ARNs](
http://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html).  For example, an AWS EC2 instance may have
a provider ID of `arn:aws:ec2:us-west-1:123456789012:instance/i-0c5192a1d67810e1a` assigned to it.  Its Lumi URN, on
the other hand, might be `urn:lumi:prod/acmecorp::acmeinfra::index::aws:ec2/instance:Instance::master-node`.  The
provider ID is a physical moniker while the Lumi URN is a logical, more stable, moniker.

Naming is trickier than it may seem.  Lumi must be able to correlate nodes in the "new" shape of a program's
resource graph with nodes in the "old" shape, for purposes of reads, updates, and deletes.  (And so Lumi doesn't
erroneously delete and recreate resources due to simple code refactorings.)  However, a Lumi program may be updated
after its initial provisioning, altering the graph's shape in a way that confuses automatic naming and correlation.

As a result, we wish the Lumi URN to be reasonably stable in the face of refactoring.  This is done by concatenating
aspects of the scope in which a resource object is allocated.  As you can see above, there are many parts to one:

* The standard URN prefix with Lumi namespace: `urn:lumi`.
* A series of tokens in the URN name, delimited by the `::` sequence of characters:
    - The stack name: `prod/acmecorp`.
    - The executable package responsible for creating this stack: `acmeinfra`.
    - The module within this package that created the resource: `index`.
    - The type of resource: `aws:ec2/instance:Instance`.
    - The friendly name of the resource assigned in the package source code: `master-node`.

Despite this relatively specific context, it is possible for the same logical resource's name to change due to simple
refactoring.  For instance, if we move a resource allocation from inside of one module and into another one, its name
will change.  It is difficult for Lumi to automatically understand such situations; that said, there is a
`pulumi rename <old> <new>` command that will rename an old URN to a new one and move all old references accordingly.  The
deployment command also attempts to recognize and suggest potential moves, although it won't perform them automatically.

## Resource Providers and Extensibility

Each resource provider plugin corresponds to a single package and handles one or more resource types.  The rule for
loading a plugin is quite simple: a binary of the name `pulumi-resource-<pkg>` is loaded, either from the path, or from
one of [the standard installation locations](deps.md).  `<pkg>` is the package token with any `/`s replaced with `_`s.

There is no requirement around a resource provider, other than that it implement a specific HTTP/2 protocol.  This
protocol is described by a [set of gRPC interfaces](
https://github.com/pulumi/pulumi/blob/master/sdk/proto/provider.proto).  In particular:

    service ResourceProvider {
        // Check validates that the given property bag is valid for a resource of the given type.
        rpc Check(CheckRequest) returns (CheckResponse) {}
        // Name names a given resource.  Sometimes this will be assigned by a developer, and so the provider
        // simply fetches it from the property bag; other times, the provider will assign this based on its own
        // algorithm.  In any case, resources with the same name must be safe to use interchangeably with one another.
        rpc Name(NameRequest) returns (NameResponse) {}
        // Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
        // must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
        rpc Create(CreateRequest) returns (CreateResponse) {}
        // Read reads the instance state identified by ID, returning a resource object, or an error if not found.
        rpc Read(ReadRequest) returns (ReadResponse) {}
        // Update updates an existing resource with new values.
        rpc Update(UpdateRequest) returns (google.protobuf.Empty) {}
        // UpdateImpact checks what impacts a hypothetical update will have on the resource's properties.
        rpc UpdateImpact(UpdateRequest) returns (UpdateImpactResponse) {}
        // Delete tears down an existing resource with the ID.  If it fails, the resource is assumed to still exist.
        rpc Delete(DeleteRequest) returns (google.protobuf.Empty) {}
    }

TODO[pulumi/pulumi#54]: eventually we will add operational APIs to resource providers.  This will allow them to report
    logging and performance counters, for example.  This will permit easy inspection from the command line (e.g.,
    `pulumi resource perf cpu <resource-id>` could print out the current CPU perf-counter history for a resource).  It
    will also permit handy resource monitoring from our hosted service, possibly in conjunction with workflows.

Describing the full RPC interface here is outside of the scope of this document.  However, all existing resources are
addressed by the resource type token plus a provider ID.  All properties are encoded as dynamically typed bags.

Eventually, we envision LumiPacks can contain both the LumiPack/LumiIL descriptions of resources, plus the associated
resource provider logic, alongside one another and in a consistent language.

There is no reason the two need to be the same, although undoubtedly that will be the most convenient approach.  Due to
the decoupling afforded by the gRPC interface, however, the toolchain takes no stance on this topic.  As such, it is
common in today's code to author resource providers in Go, where the full Lumi SDK is available to them.

