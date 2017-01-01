# Mu Languages

Mu cloud topologies are described to the toolchain using three language formats.

At the highest level, developers write Mu modules using a high-level language.  There are multiple languages to choose
from, and each is a proper subset of an existing popular programming language.  MuJS is a subset of JavaScript, MuPy is
a subset of Python, MuRu is a subset of Ruby, and MuGo is a subset of Go, for example.  The restrictions placed on these
languages are simply to ensure static analyzability, determinism, and compilability into an intermediate form.  To
distinguish between these and their ordinary counterparts, we call these Mu Metadata Languages (MetaMus).

In the middle, a packaging format, Mu Package Metadata (MuPack), is a standard metadata representation for a MuPackage.
This is the unit of distribution in the classical package management sense.  MuPack is inherently multi-langauge and
contains both internal and exported modules, types, functions, and variables.  All code is serialized in an intermediate
language, MuIL, that is suitable for interpretation for the Mu toolchain.  Because of computations, the final "shape" of
the cloud topology cannot yet be determined until the MuPackage is evaluated as part of a deployment planning step.

The final shape, Mu Graph Language (MuGL), represents a complete cloud topology with concrte property values.  Any graph
can be compared to any other graph to compute a delta, a capability essential to incremental deployment and drift
analysis.  Each graph is [directed and acyclic](https://en.wikipedia.org/wiki/Directed_acyclic_graph) (DAG), in which
nodes are cloud services, edges are [directed dependencies](https://en.wikipedia.org/wiki/Dependency_graph) between
services, and all input and output properties are known.  Any given MuPackage can create many possible MuGL graphs,
and identical MuGL graphs can be created by different MuPackages, because MuPackage can contain parameterized logic and
conditional computations.  A MuGL graph can also be generated from a live environment.

This document describes the various language concepts at play, the requirements for a high-level Mu language (although
details for each language are specified elsehwere), the MuPack and MuGL formats, and the overall compilation process.

## Mu Metadata Languages (MetaMus)

We envision a collection of high-level languages so IT professionals and developers can pick the one they feel most
comfortable with.  For example, we currently plan to support JavaScript (MuJS), Python (MuPy), Ruby (MuRu), and Go
(MuGo).  Furthermore, we imagine translators from other cloud topology formats like AWS CloudFormation and Hashicorp
Terraform.  These are called metadata languages, or MetaMus, and we call code written in them *programs*.

In principle, there is no limit to the breadth of MetaMus that Mu can support -- and it is indeed extensible by 3rd
parties -- although we do require that any MetaMu compiles down into MuPack.  This is admittedly a bit more difficult
for fully dynamically typed languages -- for example, it requires a real compiler to analyze and emit code statically --
although the task is certainly not impossible (as evidenced by MuJS, MuPy, and MuRu support).

The restrictions placed on MetaMus streamline the task of producing cloud topology graphs, and ensure that programs
are deterministic.  Determinism is important, otherwise two deployments from the exact same source programs might
result in two graphs that differ in surprising and unwanted ways.  Evaluation of the the same program must be
idempotent so that graphs and target environments can easily converge and so that failures can be dealt with reliably.

In general, this means MetaMus may not perform these actions:

* I/O of any kind (network, file, etc).
* Syscalls (except for those excplicitly blessed as being deterministic).
* Invocation of non-MetaMu code (including 3rd party packages).
* Any action that is not transitively analyable through global analysis (like C FFIs).

Examples of existing efforts to define such a subset in JavaScript, simply as an illustration, include: [Gatekeeper](
https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/gatekeeper_tr.pdf), [ADsafe](
http://www.adsafe.org/), [Caja](https://github.com/google/caja), [WebPPL](http://dippl.org/chapters/02-webppl.html),
[Deterministic.js](https://deterministic.js.org/), and even JavaScript's own [strict mode](
https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Strict_mode).  There are also multiple attempts to
catalogue sources of nondeterminism in [JavaScript](
https://github.com/burg/timelapse/wiki/Note-sources-of-nondeterminism) and [its variants](
https://github.com/WebAssembly/design/blob/master/Nondeterminism.md).

MetaMus may in fact consume 3rd party packages (e.g., from NPM, Pip, or elsewhere), but they must be blessed by the MetaMu
compiler for your language of choice.  This means recompiling packages from source -- and dealing with the possibility
that they will fail to compile if they perform illegal operations -- or, preferably, using a package that has already
been pre-compiled using a MetaMu compiler, likely in MuPack format, in which case you are guaranteed that it will work
without any unexpected difficulties.  The Mu Hub contains precompiled packages in this form for easy consumption.

Each MetaMu program is *compiled* into a MuPackage, encoded in MuPack.

## Mu Package Metadata (MuPack)

Each MuPackage is encoded in MuPack and serialized in a JSON/YAML form for easy toolability.

MuPack is the unit of sharing and reuse, and includes high-level metadata about the module's contents, in addition to
its modules, types, functions, and variables.  Each MuPackage can be either a "library" -- purely meant for sharing and
reuse purposes -- or it can be an "executable" -- in which case it can create a MuGL cloud topology all on its own.

MuPack uses a "JSON-like" type system so that its type system is accessible to many MetaMus.  This eases Internet-scale
interoperability and facilitates cross-language reuse.  This type system may be extended with custom types, including
custom service types that encapsulate patterns of infrastructure coordination, and schema types that govern the shape of
data and property values being exchanged.  Any of these types and functions may or may not be exported from the module.

All executable code is encoded using a simple intermediate language, MuIL, that is interpreted by the Mu toolchain.
This captures a deterministic, bounded set of type system and execution constructs that a subset of most higher level
languages can target and consume.  The design has been inspired by existing "minimalistic" multi-language intermediate
formats, and is very similar to [CIL](https://www.ecma-international.org/publications/standards/Ecma-335.htm), with
elements of [asm.js](http://asmjs.org/spec/latest/) and [WebAssembly](https://github.com/WebAssembly/) mixed in.

This IL is fully bound, so that IL processing needn't re-parse, re-analyze, or re-bind the resulting trees.  This has
performance advantages and simplifies the toolchain.  An optional verifier can check to ensure ASTs are well-formed.

The full MuPack specification is available [here](mupack.md).

There are two actions that can be taken against an executable MuPackage, both resulting in a MuGL graph:

* A MuPack may generate a *plan*, which is a form of graph that doesn't reflect an actual deployment yet.  Instead, it
  is essentially a "dry run" of the deployment process.  To create a plan, the MuPack's main entrypoint must be invoked,
  meaning that all of its arguments must be supplied.  Because this is a dry run, a plan's graph may be incomplete.

* A plan may then be *applied* through a similar process.  The only difference between a plan and the application of
  that plan is that, if the plan contains dependencies on output properties from services   that are to be created,
  those values are obviously unknown a priori.  Therefore, the plan might contain "holes", which will be shown in the
  plan output.  The most subtle aspect of this is that, thanks to conditional execution, the plan may in fact not just
  have holes in the values, but also uncertainty around specifically which services will be created or updated.  The
  application process performs the physical deployment steps, so all outputs are known before completion.

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
MuGL file still refer to the MuPackages that define abstractions, which are still resolved during evaluation.  All
module references will have been "locked" to a specific version of that MuPackage, however, for repeatability.

Edges between these nodes represent dependencies, and are therefore directed, and must be explicit.  Despite property
values potentially governing the dependencies, these are gone by the time MuGL is created.  Therefore, the translation
from MuPack to MuGL is responsible for fully specifying the set of service dependencies.

The graph is complete.  That is, even though dependencies on 3rd party modules may remain, the full [transitive
closure](https://en.wikipedia.org/wiki/Transitive_closure) of services created by all MuPackages is present.
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

TODO[marapongo/mu#30]: queryability (GraphQL?  RDF/SPARQL?  Neo4j/Cypher?  Gremlin?  Etc.)

## Scenarios

In this section, we'll walk through a few motivational scenarios beyond the usual compilation process from a high-level
MetaMu, to MuPack, all the way to MuGL which is deployed to an environment.  We will see how the file formats are used.

### Generating MuGL from a Live Environment

An existing environment can be used to generate MuGL.  This is called *graph inference*.

This can make adoption of Mu easier if you already have an environment you wish to model.  It can also facilitate
identifying "drift" between a desired and actual state; we will see more about this in a moment.

Any MuGL generated in this manner may have less information than MuGL generated from MetaMu and MuPack, due to the
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
staging vs. production, etc).  Alternatively, this analysis could be used to to compare an environment against a MetaMu
program's resulting MuGL, to identify places where manual changes were made to an actual environment without having
made corresponding changes in the sources.

To cope with some of the potential lossiness during graph inference, Mu implements a *semantic diff*, in addition to a
more strict exact diff, algorithm.  The semantic diff classifies differences due to lossy inference distinct from
ordinary semantically meaningful differences that could be impacting a live environment's behavior.

### Creating or Updating MetaMu and MuPack from MuGL

It is possible to raise MuGL into MuPack and, from there, raise MuPack into your favorite MetaMu.

It is, however, important to note one thing before getting into the details.  There are many possible MuPackages that
could generate a given MuGL, due to conditional execution of code.  There may even be many possible MetaMu programs 
that could generate a given MuPackage, since MuPack's language constructs are intentionally smaller than what might
exist in a higher-level programming language.  In short, lowering and re-raising is not a round-trippable operation.

Nevertheless, this raising can come in handy for two reasons.

The first is that, thanks to raising, it is possible to reconcile diffs in part by making changes to the source MetaMu
programs.  If we just altered the MuGL for a given MetaMu program, the process would be incomplete, because then
the developer would be responsible for taking that altered MuGL and translating it by hand into edits to the
program.  Automating this process as much as possible is obviously appealing even if -- similar to manual diff
resolution when applying source patches -- this process requires a little bit of manual, but tool-assistable, work.

The second helps when bootstrapping an existing environment into Mu for the first time.  Not only can we generate the
MuGL that corresponds to an existing environment, but we can generate a MetaMu in your favorite language, that will
generate an equivalent graph.  This is called *program inference*.  As with graph inference, the inference might
miss key elements like dependencies, and might not include all of the desirable abstractions and metadata, however this
can serve as a useful starting point for subsequent refactoring that would introduce such things.

