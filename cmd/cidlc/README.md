# cocogo

CocoGo compiles Go programs into Coconut metadata and packages.

## Providers

The primary use case for CocoGo is to author resource packages and providers.  A resource package is a low level
Coconut package with metadata associated with a set of resource type definitions.  Its associated provider is a dynamic
plugin that implements the behavior associated with those resources, their CRUD functions, and operational semantics.

The CocoGo toolset cuts down on boilerplate and makes it easy to author new resource packages and providers.

To generate code, run:

    $ cocogo [packages]

where the packages are a set of package names to compile.  All Go code for those packages must be legal CocoGo, a vast
subset of the overall Go programming language.  At the moment, only what is necessary for IDL is supported.

The output includes the following:

* A `Cocopack.json` package, containing all of the resource definitions.
* An output containing Go code for the resource provider:
    - A base resource provider that handles marshaling goo at the edges.
    - A marshalable type for each resource type (used for dynamic plugin serialization).

Afterwards, the Coconut package is given out to consumers that can use CocoPy, CocoRu, CocoJS, etc.  And the compiled
provider implements the behavior associated with the resource.

