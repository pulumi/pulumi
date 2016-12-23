# Mu Languages

Mu cloud topologies are described to the toolchain using three language formats.

At the highest level, developers write Mu modules using a high-level language.  There are multiple languages to choose
from, and each is a proper subset of an existing popular programming language.  MuJS is a subset of JavaScript, MuPy is
a subset of Python, MuRu is a subset of Ruby, and MuGo is a subset of Go, for example.  The restrictions placed on these
languages are simply to ensure static analyzability, determinism, and compilability into an intermediate form.  To
distinguish between these and their ordinary counterparts, we call these Mu Metadata Languages (MuMLs).

In the middle, an intermediate form, Mu Intermediate Language (MuIL), is a standard metadata representation for a
compiled module.  It is the unit of package management.  This format is inherently multi-langauge and, in addition to
containing standard metadata elements such as types and variables, it may contain computations in the form of functions,
statements, and expressions, expressed as a serialized AST plus token tables.  Because of these computations, the final
"shape" of the cloud topology cannot yet be determined, until the MuIL is evaluated as part of a plan.

The final shape, Mu Graph Language (MuGL), represents a complete cloud topology with concrte property values.  Any graph
can be compared to any other graph to compute a delta, a capability essential to incremental deployment and drift
analysis.  Each graph is [directed and acyclic](https://en.wikipedia.org/wiki/Directed_acyclic_graph) (DAG), in which
nodes are cloud services, edges are [directed dependencies](https://en.wikipedia.org/wiki/Dependency_graph) between
services, and all input and output properties are known.  Any given MuIL module can create many possible MuGL graphs,
and identical MuGL graphs can be created by different MuIL modules, because MuIL can contain parameterized logic and
conditional computations.  A MuGL graph can also be generated from a live environment.

This document describes the various language concepts at play, the requirements for a high-level Mu language (although
details for each language are specified elsehwere), the MuIL and MuGL formats, and the overall compilation process.

## Mu Metadata Languages (MuMLs)

We envision a collection of high-level languages so IT professionals and developers can pick the one they feel most
comfortable with.  For example, we currently plan to support JavaScript (MuJS), Python (MuPy), Ruby (MuRu), and Go
(MuGo).  Furthermore, we imagine translators from other cloud topology formats like AWS CloudFormation and Hashicorp
Terraform.  These are called metadata languages, or MuMLs, and we call code written in them *descriptions*.

In principle, there is no limit to the breadth of MuMLs that Mu can support -- and it is indeed extensible by 3rd
parties -- although we do require that any MuML compiles down into MuIL.  This is admittedly a bit more difficult for
fully dynamically typed languages -- for example, it requires devirtualization and therefore global analysis -- although
the task is certainly not impossible (as evidenced by MuJS, MuPy, and MuRu support).

The restrictions placed on MuMLs streamline the task of producing cloud topology graphs, and ensure that descriptions
are deterministic.  Determinism is important, otherwise two deployments from the exact same source descriptions might
result in two graphs that differ in surprising and unwanted ways.  Evaluation of the the same description must be
idempotent so that graphs and target environments can easily converge and so that failures can be dealt with reliably.

In general, this means MuMLs may not perform these actions:

* I/O of any kind (network, file, etc).
* Syscalls (except for those excplicitly blessed as being deterministic).
* Invocation of non-MuML code (including 3rd party packages).
* Any action that is not transitively analyable through global analysis (like C FFIs).

Examples of existing efforts to define such a subset in JavaScript, simply as an illustration, include: [Gatekeeper](
https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/gatekeeper_tr.pdf), [ADsafe](
http://www.adsafe.org/), [Caja](https://github.com/google/caja), [WebPPL](http://dippl.org/chapters/02-webppl.html),
[Deterministic.js](https://deterministic.js.org/), and even JavaScript's own [strict mode](
https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Strict_mode).  There are also multiple attempts to
catalogue sources of nondeterminism in [JavaScript](
https://github.com/burg/timelapse/wiki/Note-sources-of-nondeterminism) and [its variants](
https://github.com/WebAssembly/design/blob/master/Nondeterminism.md).

MuMLs may in fact consume 3rd party packages (e.g., from NPM, Pip, or elsewhere), but they must be blessed by the MuML
compiler for your language of choice.  This means recompiling packages from source -- and dealing with the possibility
that they will fail to compile if they perform illegal operations -- or, preferably, using a package that has already
been pre-compiled using a MuML compiler, likely in MuIL format, in which case you are guaranteed that it will work
without any unexpected difficulties.  The Mu Hub contains precompiled packages in this form for easy consumption.

Each MuML description is *compiled* into a MuIL module.

## Mu Intermediate Language (MuIL)

Each Mu *module* is represented in MuIL.  This format includes high-level metadata about the module's contents.  All
functions, statements, and expressions are also encoded in MuIL using a simple [abstract syntax tree](
https://en.wikipedia.org/wiki/Abstract_syntax_tree) (AST) [intermediate representation](
https://en.wikipedia.org/wiki/Intermediate_representation) (IR).  MuIL is currently serialized as JSON or YAML.

MuIL's AST is slightly higher level than a classical IR, and is closer to a real "language" (somewhat resembling an
[MIR](https://blog.rust-lang.org/2016/04/19/MIR.html) in other languages).  This AST is fully bound using a token table
approach, so that MuIL processors do not need to re-parse, re-analyze, or re-bind the resulting trees.  This has
performance advantages and simplifies the toolchain.  An optional verifier can check to ensure ASTs are well-formed.

Each MuIL module may also declare custom types and functions.  MuIL uses a "JSON-like" type system so that its universe
of types is accessible to the lowest common denominator amongst MuMLs.  This is common for Internet data exchange
already and facilitates cross-language composability.  This type system may be extended with custom types, including
service types to encapsulate patterns of service instantiations, and schema types to govern the shape of data and
property values.  Any of these types and functions may or may not be exported from the module.

MuIL is the unit of module sharing and reuse.  Although the MuMLs exist to make creating such modules easier -- as you
would typically not want to write out MuIL by hand -- each language is just convenient syntactic sugar, in a sense.

Below is a full listing of the available MuIL node types.  It captures a deterministic, bounded set of useful static
constructs that a subset of most higher level languages can easily target.  The design has been inspired by existing
"minimal AST" efforts, like [asm.js](http://asmjs.org/spec/latest/), among others.

TODO: describe the AST.

*NOTE: At this stage in the project, we are taking a shortcut, and starting with [ESTree](
https://github.com/estree/estree) as a serialized AST format.  This is slightly more convenient because we are starting
with MuJS as our first language.  We know, however, that ESTree will not be sufficient; it must be expanded to include
bound information, including the token tables (inspired by [CIL](
https://www.ecma-international.org/publications/standards/Ecma-335.htm)), and most likely we will carve out the subset
that makes sense instead of going for exhaustive support.  This will be documented here as we evolve our approach.

TODO: describe the types and type system.

TODO: a complete file format specification.

There are two actions that are taken against a MuIL module (aside from just depending on them from other MuIL modules),
both of which entails translating it into a MuGL graph:

* A module may be used to generate a *plan*, which is a form of graph that doesn't reflect an actual deployed
  environment.  To create a plan, any unbound property values from the MuIL module, if any, must be provided.  The act
  of providing such values is called *instantiation*.  Note that, as we will see, a plan's graph may be incomplete.

* A plan may then be *applied* through a similar instantiation process.  The only difference between a plan and the
  application of that plan is that, if the plan contains dependencies on output properties from services   that are to
  be created, those values are obviously unknown a priori.  Therefore, the plan might contain "holes", which will be
  shown in the plan output.  The most subtle aspect of this is that, thanks to conditional execution, the plan may in
  fact not just have holes in the values, but also uncertainty around specifically which services will be created or
  updated.  The application process performs the physical deployment steps, so all outputs are known before completion.

The result of both steps is a MuGL graph, one being more complete than the other.

## Mu Graph Language (MuGL)

MuGL is the simplest and final frontier of Mu's languages.  Each MuGL artifact -- something we just call a *graph* --
can be an in-memory data structure and/or a serialized JSON or YAML document.  It contains a graph in which each node
represents a service, each edge is a dependeny between services, and each input and output property value in the graph
is a concrete, known value, and no unresolved computations are present in the graph (holes notwithstanding).

Each graph represents the outcome of some deployment activity, either planned or actually having taken place.  Subtly,
the graph is never considered the "source of truth"; only the corresponding live running environment can be the source
of truth.  Instead, the graph describes the *intended* eventual state that a deployment activity is meant to achieve.  A
process called *reconciliation* may be used to compare differences between the two -- either on-demand or as part of a
continuous deployment process -- and resolve any differences as appropriate (through updates in either direction).

Each node in a graph carries the service's unique ID, human-friendly name, type, and its set of named property values.

A service's type tells the MuGL toolchain how to deal with physical resources that need to be created, read, updated, or
deleted, and governs which properties are legal and their expected types.  Note that any module references within the
MuGL file still refer to the MuIL-based modules files, which is still used during type and provider resolution.  All
module references will have been "locked" to a specific version of that module, however, for repeatability.

Edges between these nodes represent dependencies, and are therefore directed, and must be explicit.  Despite property
values potentially governing the dependencies, these are gone by the time MuGL is created.  Therefore, the translation
from MuIL to MuGL is responsible for fully specifying the set of service dependencies.

The graph is complete.  That is, even though dependencies on 3rd party modules may remain, the full [transitive
closure](https://en.wikipedia.org/wiki/Transitive_closure) of services created by all MuIL files is present.
Because the graph is a DAG, any cycles in this graph are illegal and will result in an error.  It is ideal if higher-
level translation catches this, since each step in the translation process reduces the diagnosability of errors.

TODO: a complete file format specification.

TODO: specify how "holes" show up during planning.

### Resource Providers

Some services are simply abstractions.  They exist solely as convenient ways to create other services, but, at the end
of the day, there are no physical manifestations of them.

Some services, however, correspond to real physical resources that must be consulted or manipulated during the planning
and/or application processes.  The extensibility mechanism used to define this logic is part of the Mu SDK and is called
a *resource provider*.  Each resource provider is associated with a Mu service type, is written in Go, and provides
standard create, read, update, and delete (CRUD) operations, that achieve the desired state changes in an environment.

Resource providers are dynamically loaded plugins that implement a standard set of interfaces.  Each resource type in Mu
must resolve to a provider, otherwise an error occurs.  Each plugin can contain multiple resource providers.

TODO: articulate the interface.

### Graph Queryability

TODO: queryability (GraphQL?  RDF/SPARQL?  Neo4j/Cypher?  Gremlin?  Etc.)

## Scenarios

In this section, we'll walk through a few motivational scenarios beyond the usual compilation process from a high-level
MuML, to MuIL, all the way to MuGL which is deployed to an environment.  We will see how the file formats are used.

### Generating MuGL from a Live Environment

An existing environment can be used to generate MuGL.  This is called *graph inference*.

This can make adoption of Mu easier if you already have an environment you wish to model.  It can also facilitate
identifying "drift" between a desired and actual state; we will see more about this in a moment.

Any MuGL generated in this manner may have less information than MuGL generated from MuML and MuIL, due to the
possibility of lossy representations and/or missing abstractions in an actual live environment.  For example, there
could be "hidden" implicit dependencies between services that are not expressed in the resulting MuGL file.
Nevertheless, this can be a great first step towards adopting Mu for your existing environments.

Generating MuGL from a live environment that was created using Mu, on the other hand, can recover all of this
information reliably, thanks to special tagging that Mu performs.

Some services map to physical artifacts in a deployment -- like a VM in your favorite cloud -- while other serivces are
simply abstractions.  In the case of abstractions, there is a limit to how much "reverse engineering" from a live
environment can happen.  The application of an abstraction merely serves to create those physical resources that are at
the "bottom" of the dependency chain.  That said, mechanisms exist to augment an environment with metadata.

### Comparing Two MuGLs

A primary feature of MuGLs is that two of them can be compared to produce a diff.  This has several use cases.

Mu performs a diff between two MuGL files to determine a delta for purposes of incremental deployment.  This allows it
to change the live environment only where a difference between actual and desired state exists.

As seen above, MuGL can be generated from a live environment.  As such, a live environment can be compared to another
MuGL file -- perhaps generated from another live environment -- to determine and reconcile "drift" between them.  This
could be used to discover differences between environments that are meant to be similar (e.g., in different zones,
staging vs. production, etc).  Alternatively, this analysis could be used to to compare an environment against a MuML
description's resulting MuGL, to identify places where manual changes were made to an actual environment without having
made corresponding changes in the sources.

To cope with some of the potential lossiness during graph inference, Mu implements a *semantic diff*, in addition to a
more strict exact diff, algorithm.  The semantic diff classifies differences due to lossy inference distinct from
ordinary semantically meaningful differences that could be impacting a live environment's behavior.

### Creating or Updating MuML and MuIL from MuGL

It is possible to raise MuGL into MuIL and, from there, raise MuIL into your favorite MuML.

It is, however, important to note one thing before getting into the details.  There are many possible MuIL modules that
could generate a given MuGL, due to conditional execution of code.  There may even be many possible MuML descriptions
that could generate a given MuIL, since MuIL's language constructs are intentionally smaller than what might exist in a
higher-level programming language.  In short, lowering and re-raising is not a round-trippable operation.

Nevertheless, this raising can come in handy for two reasons.

The first is that, thanks to raising, it is possible to reconcile diffs in part by making changes to the source MuML
descriptions.  If we just altered the MuGL for a given MuML description, the process would be incomplete, because then
the developer would be responsible for taking that altered MuGL and translating it by hand into edits to the
description.  Automating this process as much as possible is obviously appealing even if -- similar to manual diff
resolution when applying source patches -- this process requires a little bit of manual, but tool-assistable, work.

The second helps when bootstrapping an existing environment into Mu for the first time.  Not only can we generate the
MuGL that corresponds to an existing environment, but we can generate a MuML in your favorite language, that will
generate an equivalent graph.  This is called *description inference*.  As with graph inference, the inference might
miss key elements like dependencies, and might not include all of the desirable abstractions and metadata, however this
can serve as a useful starting point for subsequent refactoring that would introduce such things.

