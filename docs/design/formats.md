# Coconut Formats

Coconut cloud services are described to the toolchain with three languages: CocoLangs, CocoPack/CocoIL, and CocoGL.

At the highest level, developers write Coconut packages using a high-level language.  There are multiple to choose from,
and each is a proper subset of an existing popular programming language; CocoJS is a subset of JavaScript, CocoPy is
a subset of Python, CocoRu is a subset of Ruby, and CocoGo is a subset of Go, for example.  The restrictions placed on
these languages ensure static analyzability, determinism, and compilability into an intermediate form.  That said, they
retain most of their source language's popular features, including the libraries, so that these languages feel familiar.
To distinguish them from their ordinary counterparts, these subsetted language dialects are called CocoLangs.

In the middle, CocoPack is a standard metadata representation for compiled and redistributable Coconut packages.  This is
the unit of distribution and dependency in the classical package management sense.  CocoPack is multi-langauge and
contains internal and exported modules, types, functions, variables, and code.  All code is serialized in CocoIL, the
CocoPack intermediate language, that can be evaluated by the Coconut toolchain.  Because of computations, the final
"shape" of a cloud topology is not determined until the CocoPack is evaluated as part of a deployment planning step.

The final shape, Coconut Graph Language (CocoGL), represents a cloud topology with concrete property values and
dependencies.  A graph can be compared to another graph to compute a delta, a capability essential to incremental
deployment and drift analysis.  Each graph is a [DAG](https://en.wikipedia.org/wiki/Directed_acyclic_graph), in which
nodes are cloud services, edges are [directed dependencies](https://en.wikipedia.org/wiki/Dependency_graph) between
services, and all input and output properties are values.  Any given CocoPack can create many possible CocoGL graphs,
and identical CocoGL graphs can be created by different CocoPacks, because CocoPack can contain parameterized logic and
conditional computations.  A CocoGL graph can also be generated from a live environment.

This document describes these formats, the requirements for a high-level CocoLang (although details for each language
are specified elsehwere), the CocoPack/CocoIL and CocoGL formats, and the overall compilation process.

## Coconut Metadata Languages (CocoLangs)

We envision a collection of high-level languages so IT professionals and developers can pick the one they feel most
comfortable with.  For example, we currently plan to support JavaScript (CocoJS), Python (CocoPy), Ruby (CocoRu), and Go
(CocoGo).  Furthermore, we imagine translators from other cloud topology formats like AWS CloudFormation and Hashicorp
Terraform.  These are called metadata languages, or CocoLangs, and we call code written in them *programs*.

In principle, there is no limit to the breadth of CocoLangs that Coconut can support -- and it is indeed extensible by
3rd parties -- although we do require that any CocoLang compiles down into CocoPack.  This is admittedly a bit more
difficult for fully dynamically typed languages -- for example, it requires a real compiler to analyze and emit code
statically -- although the task is certainly not impossible (as evidenced by CocoJS, CocoPy, and CocoRu support).

### Not Your Average Language

It is important to reiterate that CocoLangs are not traditional languages.  Programs describe the desired state of a
cloud service or collection of cloud services.  The evaluation of a CocoLang program results in a CocoGL DAG that captures
dependencies and state that correspond to physical entities in a target cloud environment.  The Coconut toolset then
takes this DAG, compares it to the existing environment, and "makes it so."

Note that this approach fully embraces the [immutable infrastructure](
http://martinfowler.com/bliki/ImmutableServer.html) philosophy, including embracing [cattle over pets](
https://blog.engineyard.com/2014/pets-vs-cattle).

A CocoLang program itself is just metadata, therefore, and any computations in the language itself exist solely to
determine this DAG.  This is a subtle but critical point, so it bears repeating: CocoLang code does not actually execute
within the target cloud environment; instead, CocoLang code merely describes the topology of the code and resources.

It is possible to mix CocoLang and regular code in the respective source language.  This is particularly helpful when
associating runtime code to deployment-time artifacts, and is common when dealing with serverless programming.  For
example, instead of needing to manually separate out the description of a cloud lambda and its code, we can just write a
lambda that the compiler will "shred" into a distinct asset that is bundled inside the cloud deployment automatically.

In any case, "language bindings" bind elements of Coconut services to executable code.  This executable code can come in
many forms, aside from the above lambda example.  For example, a "container" service may bind to a real, physical Docker
container image.  As another example, an "RPC" service may bind to an entire Go program, with many endpoints implemented
as Go functions.  A CocoPack is incomplete without being fully bound to the program assets that must be co-deployed.

### Restricted Subsets

The restrictions placed on CocoLangs streamline the task of producing cloud topology graphs, and ensure that programs
are deterministic.  Determinism is important, otherwise two deployments from the exact same source programs might
result in two graphs that differ in surprising and unwanted ways.  Evaluation of the the same program must be
idempotent so that graphs and target environments can easily converge and so that failures can be dealt with reliably.

In general, this means CocoLangs may not perform these actions:

* I/O of any kind (network, file, etc).
* Syscalls (except for those excplicitly blessed as being deterministic).
* Invocation of non-CocoLang code (including 3rd party packages).
* Any action that is not transitively analyable through global analysis (like C FFIs).

Examples of existing efforts to define such a subset in JavaScript, simply as an illustration, include: [Gatekeeper](
https://www.microsoft.com/en-us/research/wp-content/uploads/2016/02/gatekeeper_tr.pdf), [ADsafe](
http://www.adsafe.org/), [Caja](https://github.com/google/caja), [WebPPL](http://dippl.org/chapters/02-webppl.html),
[Deterministic.js](https://deterministic.js.org/), and even JavaScript's own [strict mode](
https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Strict_mode).  There are also multiple attempts to
catalogue sources of nondeterminism in [JavaScript](
https://github.com/burg/timelapse/wiki/Note-sources-of-nondeterminism) and [its variants](
https://github.com/WebAssembly/design/blob/master/Nondeterminism.md).

CocoLangs may in fact consume 3rd party packages (e.g., from NPM, Pip, or elsewhere), but they must be blessed by the
CocoLang compiler for your language of choice.  This means recompiling packages from source -- and dealing with the
possibility that they will fail to compile if they perform illegal operations -- or, preferably, using a package that
has already been pre-compiled using a CocoLang compiler, likely in CocoPack format, in which case you are guaranteed that it
will work without any unexpected difficulties.  The CocoHub contains precompiled CocoPacks for easy consumption.

Each CocoLang program is *compiled* into a CocoPack, using the NuPack and CocoIL formats.

## Coconut Package Metadata (CocoPack) and Intermediate Language (CocoIL)

Each CocoPack is encoded in CocoPack and serialized in a JSON/YAML form for easy toolability.  The full CocoPack and CocoIL
specifications are available in the [Coconut Package Metadata (CocoPack) design doc](nutpack.md).

### CocoPack

CocoPack is the unit of sharing and reuse, and includes high-level metadata about the module's contents, in addition to
its modules, types, functions, and variables.  Each CocoPack can be either a *library* -- meant solely for sharing and
reuse -- or it can be an executable *blueprint* -- in which case it can create a CocoGL cloud topology all on its own.

CocoPack uses a "JSON-like" type system so that its type system is accessible to many CocoLangs.  This eases Internet-scale
interoperability and facilitates cross-language reuse.  This type system may be extended with custom types, including
custom service types that encapsulate patterns of infrastructure coordination, and schema types that govern the shape of
data and property values being exchanged.  Any of these types and functions may or may not be exported from the module.

### CocoIL

All executable code is encoded using a simple intermediate language, CocoIL, that can be evaluated by Coconut.
This captures a deterministic, bounded set of type system and execution constructs that a subset of most higher level
languages can target and consume.  The design has been inspired by existing "minimalistic" multi-language intermediate
formats, and is very similar to [CIL](https://www.ecma-international.org/publications/standards/Ecma-335.htm), with
elements of [asm.js](http://asmjs.org/spec/latest/) and [WebAssembly](https://github.com/WebAssembly/) mixed in.

This IL is fully bound, so that IL processing needn't re-parse, re-analyze, or re-bind the resulting trees.  This has
performance advantages and simplifies the toolchain.  An optional verifier can check to ensure ASTs are well-formed.

### Planning and Applying

There are two actions that can be taken against a CocoPack blueprint:

* `coco plan` turns a blueprint into a *plan*, which is a possibly-incomplete graph, and does not actually mutate any
  live environment.  This is essentially a "dry run" of a deployment.  To create a plan, the CocoPack's main entrypoint
  is called, requiring that its arguments be supplied.  The graph may be incomplete if code depends on plan outputs for
  conditional execution or property values,  resulting in so-called "holes" that will be shown in the plan's output.

* `coco apply` applies a plan to a target environment.  The primary difference between generating and applying a plan is
  that real resources may be created, updated, and deleted.  Because actions are actually performed, all outputs are
  known, and so the resulting graph will be complete.  The resulting graph can be saved for future use.

The result of both steps is a CocoGL graph, the latter being strictly more complete than the former.

## Coconut Graph Language (CocoGL)

CocoGL is the simplest and final frontier of Coconut's formats.  Each CocoGL artifact -- something we just call a
*graph* -- can be an in-memory data structure and/or a serialized into JSON or YAML.  Each vertex represents a service,
each edge is a dependeny between services, and each input and output property value in the graph is a concrete, known
value, and no unresolved computations are present in the graph (holes notwithstanding).

Each graph represents the outcome of some deployment activity, either planned or actually having taken place.  Subtly,
the graph is never considered the "source of truth"; only the corresponding live running environment can be the source
of truth.  Instead, the graph describes the *intended* eventual state that a deployment activity is meant to achieve.  A
process called *reconciliation* may be used to compare differences between the two -- either on-demand or as part of a
continuous deployment process -- and resolve any differences as appropriate (through updates in either direction).

Each node in a graph carries the service's unique ID, human-friendly name, type, and its set of named property values.

A service's type tells the CocoGL toolchain how to deal with physical resources that need to be created, read, updated, or
deleted, and governs which properties are legal and their expected types.  Note that any module references within the
CocoGL file still refer to the CocoPacks that define abstractions, which are still resolved during evaluation.  All
module references will have been "locked" to a specific version of that CocoPack, however, for repeatability's sake.

Edges between these nodes represent dependencies, and are therefore directed, and must be explicit.  Despite property
values potentially governing the dependencies, these are gone by the time CocoGL is created.  Therefore, the translation
from CocoPack to CocoGL is responsible for fully specifying the set of service dependencies.

The graph is complete.  That is, even though dependencies on 3rd party modules may remain, the full [transitive
closure](https://en.wikipedia.org/wiki/Transitive_closure) of services created by all CocoPacks is present.
Because the graph is a DAG, any cycles in this graph are illegal and will result in an error.  It is ideal if higher-
level translation catches this, since each step in the translation process reduces the diagnosability of errors.

TODO: a complete file format specification.

TODO: specify how "holes" show up during planning ("<computed>").  E.g., do we simulate control flow paths.

TODO: describe the algorithm used for diffing two CocoGLs.

TODO: describe what happens in the face of partial application failure.  Do graphs become tainted?

### Resource Providers

In general, the Coconut toolset simply performs an in-order walk of the CocoGL DAG in order to perform a deployment.
However, clearly there must be more to this, in order to actually perform mutations in the target environment.

Some services are simply abstractions.  They exist solely as convenient ways to create other services, but, at the end
of the day, there are no physical manifestations of them.  Some services, however, correspond to real physical resources
that must be consulted or manipulated during the planning and/or application processes.  These are called *resources*.

The extensibility mechanism used to define this logic is part of the Coconut SDK and is called a *resource provider*.
Each resource provider is associated with a Coconut service type, usually written in Go, and provides standard create,
read, update, and delete (CRUD) operations, that achieve the desired state changes in an environment.

Resource providers are dynamically loaded plugins that implement a standard set of interfaces.  Each resource type in
Coconut must resolve to a provider, otherwise an error occurs.  Each plugin can contain multiple resource providers.

Please refer to the [resource extensibility design doc](resources.md) for more details on this model.

### Graph Queryability

TODO[pulumi/coconut#30]: queryability (GraphQL?  RDF/SPARQL?  Neo4j/Cypher?  Gremlin?  Etc.)

## Scenarios

In this section, we'll walk through a few motivational scenarios beyond the usual compilation process from a high-level
CocoLang, to CocoPack, all the way to CocoGL which is deployed to an environment.  We will see how the file formats are used.

### Generating CocoGL from a Live Environment

An existing environment can be used to generate CocoGL.  This is called *graph inference*.

This can make adoption of Coconut easier if you already have an environment you wish to model.  It can also facilitate
identifying "drift" between a desired and actual state; we will see more about this in a moment.

Any CocoGL generated in this manner may have less information than CocoGL generated from CocoLang and CocoPack, due to the
possibility of lossy representations and/or missing abstractions in an actual live environment.  For example, there
could be "hidden" implicit dependencies between services that are not expressed in the resulting CocoGL file.
Nevertheless, this can be a great first step towards adopting Coconut for your existing environments.

Generating CocoGL from a live environment that was created using Coconut, on the other hand, can recover all of this
information reliably, thanks to special tagging that Coconut performs.

Some services map to physical artifacts in a deployment -- like a VM in your favorite cloud -- while other serivces are
simply abstractions.  In the case of abstractions, there is a limit to how much "reverse engineering" from a live
environment can happen.  The application of an abstraction merely serves to create those physical resources that are at
the "bottom" of the dependency chain.  That said, mechanisms exist to augment an environment with metadata.

### Comparing Two CocoGLs

A primary feature of CocoGLs is that two of them can be compared to produce a diff.  This has several use cases.

Coconut performs a diff between two CocoGL files to determine a delta for purposes of incremental deployment.  This
allows it to change the live environment only where a difference between actual and desired state exists.

As seen above, CocoGL can be generated from a live environment.  As such, a live environment can be compared to another
CocoGL file -- perhaps generated from another live environment -- to determine and reconcile "drift" between them.  This
could be used to discover differences between environments that are meant to be similar (e.g., in different zones,
staging vs. production, etc).  Alternatively, this analysis could be used to to compare an environment against a CocoLang
program's resulting CocoGL, to identify places where manual changes were made to an actual environment without having
made corresponding changes in the sources.

To cope with some of the potential lossiness during graph inference, Coconut implements a *semantic diff*, in addition
to a more strict exact diff, algorithm.  The semantic diff classifies differences due to lossy inference distinct from
ordinary semantically meaningful differences that could be impacting a live environment's behavior.

### Creating or Updating CocoLang and CocoPack from CocoGL

It is possible to raise CocoGL into CocoPack and, from there, raise CocoPack into your favorite CocoLang.

It is, however, important to note one thing before getting into the details.  There are many possible CocoPacks that
could generate a given CocoGL, due to conditional execution of code.  There may even be many possible CocoLang programs 
that could generate a given CocoPack, since CocoPack's language constructs are intentionally smaller than what might
exist in a higher-level programming language.  In short, lowering and re-raising is not a round-trippable operation.

Nevertheless, this raising can come in handy for two reasons.

The first is that, thanks to raising, it is possible to reconcile diffs in part by making changes to the source CocoLang
programs.  If we just altered the CocoGL for a given CocoLang program, the process would be incomplete, because then
the developer would be responsible for taking that altered CocoGL and translating it by hand into edits to the
program.  Automating this process as much as possible is obviously appealing even if -- similar to manual diff
resolution when applying source patches -- this process requires a little bit of manual, but tool-assistable, work.

The second helps when bootstrapping an existing environment into Coconut for the first time.  Not only can we generate
the CocoGL that corresponds to an existing environment, but we can generate a CocoLang in your favorite language, that
will generate an equivalent graph.  This is called *program inference*.  As with graph inference, the inference might
miss key elements like dependencies, and might not include all of the desirable abstractions and metadata, however this
can serve as a useful starting point for subsequent refactoring that would introduce such things.

