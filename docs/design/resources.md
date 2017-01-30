# Resources

Mu describes cloud resources and programs for purposes of provisioning, updating, and deleting.  From most of the other
MuPack and MuIL documentation, it won't yet be clear how this happens.  The MuPack and MuIL formats are intentionally
general purpose and themselves contain no specific understanding of cloud resources.  This design note fills that gap.

## Overview

Each MuPackage contains program logic that encodes control flow and data creation.  This data comes in the form of the
usual programming language runtime constructs, namely primitive data (numbers, strings, etc) and objects.  For example:

    let bucket = new s3.Bucket({ deletionPolicy: "retain" });

The evaluation of such a MuIL program, and the resulting runtime objects and data, drives the creation of a MuGL
resource graph.  This is imply a directed acyclic graph (DAG) whose nodes represent resources and whose edges represent
resource dependencies.  From this graph, the Mu toolchain can determine how to provision the desired resources.

## Identifying a Resource

For this to work, we must first identify what a "resource" is.

A resource in the programming model is simply identified by any class that ultimately derives from the `mu.Resource`
base class.  (The specific manifestation of this depends on your language SDK; examples below are in MuJS.)  This class
is primarily a marker class in that it doesn't offer anything of its own (except for a few advanced methods, see below).

    import * as mu from "mu";
    class MyResouce extends mu.Resource {
        constructor(...) {
            super(...);
            // ...
        }
        // ...
    }

To create a MuGL graph, Mu evaluates the program.  As it does so, the Mu interpreter runtime monitors object creation
and dependency formation.  The interpreter recognizes which of those objects are resources and records them.

Each resource may have any number of properties.  Some properties may form dependencies on other resources, either by
storing a reference to another resource object, or by allocating one (directly or indirectly).  These dependencies are
ultimately used to form the MuGL DAG.  In some rare circumstances, dependencies may need to be registered explicitly.

Some properties may not have values until the resulting MuGL graph is actually applied to a target environment; these
are called *provisioned properties* and will receive default values from the resource provider somehow, possibly by
contacting the target cloud provider (e.g., default VM sizes, etc).  It is possible to use these properties, however, we
must tread with care.  A provisioned property's value cannot be known during planning.  Any conditional logic that
depends upon a provisioned property would mean an inability to plan, and so is rejected by default.

Most of the magic of a true resource is available in its *resource provider* plugin.  This provider implements create,
read, update, and delete (CRUD) operations for that resource type, in addition to some optional facilities, such as
logging and operational performance counters.  These plugins are registered and loaded dynamically (see more below).

## Naming Resources

Each resource has a unique ID ("name") that enables Mu operations to address it.

Please note that this name doesn't need to be equivalent to the name or ID of the resource in the target environment.
For example, AWS resources will have [ARNs](http://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html)
that are distinct from Mu's name.  Mu maintains a mapping from its own names to these authoritative names.

Naming is trickier than it may seem.  Mu must be able to correlate nodes in the "new" shape of the program's resource
graph with nodes in the "old" shape, for purposes of reads, updates, and deletes.  (And so Mu don't erroneously delete
and recreate resources due to simple code refactorings.)  However, a Mu program may get updated after the initial
provisioning of an environment, altering the graph's shape in a way that confuses automatic naming and correlation.

The default name is formed by concatenating the scope in which a resource object is allocated.  This scope is formed
from the module and, optionally, class in which the object was allocated.  This scope is then further amended based on
the target environment name and, optionally, an additional namespace for multi-instancing.

In general, this name will be of one of these forms:

    <ENV>#GID
    <ENV>:<MODULE>:<RESOURCE>
    <ENV>:<MODULE>:<RESOURCE>#ID
    <ENV>:<MODULE>/<CLASS>:<RESOURCE>
    <ENV>:<MODULE>/<CLASS>:<RESOURCE>#ID

The `<ENV>` part is either just the environment name (e.g., `staging`) or the environment name plus namespace (e.g.,
`staging:acmecorp`).  These are controlled through metadata and/or command line arguments and are shared amongst all
resources in a given deployment.

The `#GID` is a globally unique ID.  It is possible that a resource would offer a property that maps to a globally
unique ID, in which case Mu will use it.  This provides the ability for perfectly stable names.  But it is usually more
cumbersome for users to manage such IDs and, if used incorrectly, opens up the risk for accidental collisions.  Most
users will opt instead for the default naming scheme based on the context in which a resource has been created.

If default naming is used, the `<MODULE>` part is the module token, encoding both the package and module name (e.g.,
`mu/x:cluster/utils`).  The optional `<CLASS>` part encodes the class's name in which the resource was allocated.

Finally, the `<RESOURCE>` part is the token for the resource's type.  This will include the fully qualified package,
module, and class name for the resource.  The optional `#ID` part will either be an auto-incrementing number -- in
the case that multiple such resources are created -- or an optional disambiguating name if one has been provided.  Note
that `#ID` differs from `#GID` in that it needn't be globally unique; instead, it could be a more "friendly" short name.

Despite this relatively specific context, it is possible for the same logical resource's name to change due to sikmple
refactoring.  For instance, if we move a resource allocation from inside of one module and/or class and into another
one, its name will change.  It is difficult for Mu to automatically understand such situations.  If there is an
explicit `#ID` (not auto-incrementing number), Mu will attempt to suggest a bridge between the two names.  If there is
not, however, then a plan will, possibly surprisingly, claim that it will delete the old and create the new resources.
The `mu rename <old> <new>` command can be used to explicitly help the planner bridge the gap between the two names.

## Resource Providers and Extensibility

Each resource provider plugin corresponds to a single MuPackage and defines one or more resource implementations
written in Go, for all resources within that MuPackage.  This plugin is loaded by name and runs inside of the Mu
toolchain and runtime.  Each resource is identified dynamically by its fully qualified resource name.

The `marapongo/mu/pkg/resources/Provider` interface defines the set of operations a resource provider must define.  The
primary operations are the CRUD, and are required, while the others are optional but light up some operational features:

    type Provider interface {
        Create()     // create a new instance.
        Read()       // read the resource's current properties.
        Update()     // update the resource's updateable properties.
        Delete()     // delete the resource.

        FetchLog()   // fetches logs associated with this resource.
        FetchStats() // fetches performance counters associated with this resource.
    }

TODO: specify the final shape of all of this: types, protobufs, etc.

