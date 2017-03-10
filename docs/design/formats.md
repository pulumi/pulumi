# Coconut Formats

Coconut cloud topologies are described using three formats: CocoLangs, CocoPack/CocoIL, and CocoGL.

At the highest level, developers write Coconut packages using a high-level language.  There are multiple to choose from,
and each is a proper subset of an existing popular programming language; CocoJS is a subset of JavaScript, CocoPy is
a subset of Python, CocoRu is a subset of Ruby, and CocoGo is a subset of Go, for example.  The restrictions placed on
these languages ensure static analyzability, determinism, and compilability into an intermediate form.  That said, they
retain most of their source language's popular features, including the libraries, so that these languages feel familiar.
To distinguish them from their ordinary counterparts, these subsetted language dialects are called CocoLangs.

In the middle, CocoPack is a standard metadata representation for compiled and redistributable Coconut packages.  This
is the unit of distribution and dependency in the classical package management sense.  CocoPack is multi-langauge and
contains internal and exported modules, types, functions, variables, and code.  All code is serialized into CocoIL, the
CocoPack intermediate language, that can be evaluated by the Coconut toolchain.  Because of computations, the final
"shape" of a cloud topology is not determined until the CocoPack is evaluated as part of a deployment planning step.

The final shape, Coconut Graph Language (CocoGL), represents a cloud topology with concrete property values and
dependencies.  A graph can be compared to another graph to compute a delta, a capability essential to incremental
deployment and drift analysis.  Each graph is a [DAG](https://en.wikipedia.org/wiki/Directed_acyclic_graph), in which
nodes are cloud resources, edges are [directed dependencies](https://en.wikipedia.org/wiki/Dependency_graph) between
them, and all input and output properties are known values.  Any given CocoPack can create many possible CocoGL graphs,
and identical CocoGL graphs can be created by different CocoPacks, because CocoPack can contain parameterized logic and
conditional computations.  A CocoGL graph can also be generated from a live environment, something we call a snapshot.

This document describes these formats, the requirements for a high-level CocoLang (although details for each language
are specified elsehwere), the CocoPack/CocoIL and CocoGL formats, and the overall compilation process.

## Coconut Languages (CocoLangs)

We envision a collection of high-level languages so IT professionals and developers can pick the one they feel most
comfortable with.  For example, we currently plan to support JavaScript (CocoJS), Python (CocoPy), Ruby (CocoRu), and Go
(CocoGo).  Furthermore, we imagine translators from other cloud topology formats like AWS CloudFormation and Hashicorp
Terraform.  These are called metadata languages, or CocoLangs, and we call code artifacts written in them packages.

In principle, there is no limit to the breadth of CocoLangs that Coconut can support -- and it is indeed extensible by
3rd parties -- although we do require that any CocoLang compiles down into CocoPack.  This translation requires static
code translation -- admittedly an extra step for most dynamic languages -- however, the CocoPack/CocoIL format was
designed with these dynamic languages in mind and supports full dynamic dispatch, inspection, and so on.

### Configuration versus Runtime Code

It is important to reiterate that CocoLangs are not traditional languages.  Packages written in them describe the
desired state of a cloud topology of resources.  The evaluation of a CocoLang program results in a CocoGL DAG that
captures the intended state corresponding to physical entities in a target cloud environment, as well as dependencies
between them.  The Coconut toolset then takes this DAG, compares it to the existing environment, and "makes it so."

Note that this approach fully embraces the [immutable infrastructure](
http://martinfowler.com/bliki/ImmutableServer.html) philosophy, including embracing [cattle over pets](
https://blog.engineyard.com/2014/pets-vs-cattle).  But it leverages the full power of real programming languages.

A CocoLang package itself is just metadata, therefore, and any computations in the language itself exist solely to
determine this DAG.  This is a subtle but critical point, so it bears repeating: CocoLang code does not actually execute
within the target cloud environment, and it alone does not perform side-effects; instead, CocoLang code merely describes
the topology of the code and resources, and the Coconut toolchain orchestrates deployments based on analysis of it.

It is possible to mix CocoLang and regular code in the respective source language.  This is particularly helpful when
associating runtime code to deployment-time artifacts, and is common when dealing with serverless programming.  For
example, instead of needing to manually separate out the description of a cloud lambda and its code, we can just write a
lambda that the compiler will "shred" into a distinct asset that is bundled inside the cloud deployment automatically.

For example, this mixes configuration and runtime code:

    let func = new coconut.x.Function(ctx => {
        ctx.write("Hello, Coconut!");
    });

In this CocoJS program, the `let func = new coconut.x.Function({})` part uses the CocoJS subset.  However, the code
inside of the lambda is real JavaScript, and may use NPM packages, perform IO, and so forth.  The CocoJS compiler
understands how to compile this in a dual mode so that the resulting artifacts can be deployed independently.

In any case, "language bindings" bind certain resource properties to executable code.  This executable code can come in
many forms, aside from the above lambda example.  For example, a container resource may bind to a real, physical Docker
container image.  As another example, an RPC resource may bind to an entire Go program, with many endpoints implemented
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
has already been pre-compiled using a CocoLang compiler, likely in CocoPack format, in which case you are guaranteed
that it will work without any unexpected difficulties.  The CocoHub contains precompiled CocoPacks for easy consumption.

Each CocoLang program is compiled into a package that leverages the CocoPack and CocoIL formats.

## Coconut Package (CocoPack) and Intermediate Language (CocoIL) Formats

Each Coconut package is encoded in CocoPack and serialized in a JSON/YAML form for easy toolability.  The full CocoPack
and CocoIL specifications are available in the [Coconut Package Metadata (CocoPack) design doc](packages.md).

### CocoPack

CocoPack is the unit of sharing and reuse, and includes high-level metadata about the module's contents, in addition to
its modules, types, functions, and variables.  Each CocoPack can be either a *library* or an *executable*.  A library is
not meant to be instantiated on its own; instead, it packages up abstractions that are useful to other packages.  An
executable, on the other hand, is executed in order to create a CocoGL graph and cloud topology all on its own.

For example, the package metadata includes things like a name, description, and dependencies:

    {
        "name": "acmecorp/aws-example",
        "description": "Simple example of using AWS resources.",
        "dependencies": {
            "aws": "*",
            "coconut": "1.0",
            "cocojs": "^1.0.7"
        },
        "modules": {
            // CocoIL goes here
        }
    }

CocoPack uses a "JSON-like" type system to ease cross-language reuse and Internet interoperability.  This type system
may be extended with custom types, including custom resources, functions that encapsulate patterns of common
infrastructure logic, and schema types like enums that govern the shape of data and property values being exchanged.
Any of these types and functions may or may not be exported from a package.

### CocoIL

Executable code artifacts are encoded using a simple intermediate language, CocoIL, that Coconut may evaluate.

This captures a deterministic, bounded set of type system and execution constructs that a subset of most higher level
languages can target and consume.  The design has been inspired by existing "minimalistic" multi-language intermediate
formats, and is very similar to [Java bytecode](https://docs.oracle.com/javase/specs/jvms/se8/html/) and [.NET CIL](
https://www.ecma-international.org/publications/standards/Ecma-335.htm), with elements of [asm.js](
http://asmjs.org/spec/latest/) and [WebAssembly](https://github.com/WebAssembly/) mixed in for dynamic translation.

All elements within a package are referred to by tokens.  For example, the type token for the AWS `Instance` resource
class, within the `aws` package and `ec2/instance` module, would be `aws:ec2/instance:Instance`.

This IL is fully bound by CocoLang compilers, so that IL processing needn't re-parse, re-analyze, or re-bind the
resulting trees.  This simplifies the toolchain, particularly given that many languages have their own looking, binding,
and scoping rules that would be difficult for Coconut to perform in a language-agnostic way.  The IL is an abstract
syntax tree (AST)-based IL rather than, say, a stack machine.  This choice simplifies the creation of new CocoLang
compilers.  All order of evaluation is encoded by CocoLang compilers so that again Coconut may remain language-neutral.

An optional verifier can check to ensure ASTs are well-formed:

	$ coco pack verify

Unlike many virtual machines, Coconut aggressively performs verification during evaluation of packages.  This catches
subtle runtime errors sooner and lets dynamic languages lean on runtime verification.  Coconut is concerned more with
correctness and reliability than evaluation performance, which for cloud resources is often already measured in seconds.

### Planning and Deploying

An executable package is eventually deployed as a stack.  This takes one of two forms:

* *Plan*: `coco deploy <env> -n` generates a plan, a description of a hypothetical deployment, but does not actually
  mutate anything.  This is essentially a "dry run" to see what changes will occur.  To do this, the package's main
  entrypoint is called with the provided arguments.  The graph may be incomplete if code depends on resource outputs for
  conditional execution or property values, resulting in so-called "holes" that will be shown in the plan's output.

* *Deployment*: `coco deploy <env>` applies a plan to a target environment `<env>`.  The primary difference compared to
  a plan is that real resources may be created, updated, and deleted.  Because side-effecting actions are performed, all
  outputs are known, and so the resulting graph will be complete.  The resulting deployment record is saved locally.

The result of both steps is a CocoGL graph, the latter being strictly more complete than the former.

## Coconut Graph Language (CocoGL)

CocoGL is the simplest and final frontier of Coconut's formats.  Each CocoGL artifact -- something we just call a
graph -- can be an in-memory data structure and/or a serialized into JSON or YAML.

In its most general form, CocoGL is just a graph of objects, their properties, and their dependencies amongst one
another.  However, CocoGL is used in a very specific way when it comes to Coconut packages, plans, and deployments.

Specifically, when it comes to deployments, each vertex represents a resource object, complete with properties, and each
edge is a dependeny between resources.  Such a graph is required to be a DAG so that resource ordering is well-defined.
Each such graph represents the outcome of some deployment activity, either planned or actually having taken place.

Subtly, the graph is never considered the "source of truth"; only the corresponding live running environment can be the
source of truth.  Instead, the graph describes the *intended* eventual state that a deployment activity is meant to
achieve.  A *snapshot* may be generated from a live environment to capture the actual state of running resources as a
CocoGL resource graph.  A process called *reconciliation* compares differences between the two -- either on-demand or as
part of a continuous monitoring process -- and resolve any differences (through updates in either direction).

Each vertex in a graph carries the resource's provider-assigned ID, a URN, type, and a set of named property values.
The provider ID is opaque to Coconut, and is assigned by the resource provider, while the URN is automatically assigned.
An example AWS EC2 instance ID might be `i-0c5192a1d67810e1a` and URN
`urn:coconut:prod/acmecorp::acmeinfra::index::aws:ec2/instance:Instance::master-node`.

A resource's type tells the CocoGL toolchain how to load a plugin to deal with physical resources that need to be
created, read, updated, or deleted, and governs which properties are legal and their expected types.  Any module
references within the CocoGL file still refer to the respective packages, which are resolved during evaluation.  All
module references will have been "locked" to a specific version of that CocoPack, however, for repeatability's sake.

Edges between these nodes represent dependencies, and are therefore directed, and must be explicit.  The graph itself
fully describes the complete set of dependencies.  That is, consider a resource A's property whose value was computed by
reading the output of some other resource B; this dependency from A on B exists in code, and must therefore be
translated into an edge in the resulting graph when evaluating CocoPack/CocoIL and creating CocoGL.

By default, graphs are "complete", and statically link in the resulting graphs from dependency modules.  Even though
dependencies on 3rd party modules will remain in the form of tokens and URNs, the graph contains the full [transitive
closure](https://en.wikipedia.org/wiki/Transitive_closure) of resources created by all packages run during evaluation.
Because the graph is a DAG, any cycles in this graph are illegal and will result in an error.  It is ideal if higher-
level translation catches this, since each step in the translation process reduces the diagnosability of errors.
Dynamic linking is possible for advanced scenarios, and leverages URNs for discovery, but is described elsewhere.

Please refer to the [design document](https://github.com/pulumi/coconut/blob/master/docs/design/graphs.md) for more
information about CocoGL graphs.

### Resource Providers

In general, the Coconut toolset simply performs an in-order walk of the CocoGL DAG in order to perform a deployment.
However, clearly there must be more to this, in order to actually perform mutations in the target environment.

Some objects are simply abstractions.  They exist solely as convenient ways to create other objects, but, at the end
of the day, there are no physical manifestations of them.  Some objects, however, correspond to real physical resources
that must be consulted or manipulated during the planning and/or application processes.  These are called *resources*,
and are indicated to the Coconut runtime by subclassing from a well-known base class:

    export class MyResource extends coconut.Resource {
        ...
    }

The extensibility mechanism used to define this logic is part of the Coconut SDK and is called a *resource provider*.
Each package that defines a resource must also carry with it a corresponding resource provider plugin.  These plugins
are loaded dynamically by the Coconut runtime anytime it needs to interact with a resource in a given package.  These
plugins implement a simple inteface consisting primarily of create, read, update, and delete (CRUD) operations.

Please refer to the [resources design document](resources.md) for more details on resources and resource providers.

## Scenarios

In this section, we'll walk through a few motivational scenarios beyond the usual compilation process from a high-level
CocoLang, to CocoPack, all the way to CocoGL which is deployed to an environment, to see how the file formats are used.

### Generating CocoGL from a Live Environment

An existing environment can be used to generate a CocoGL snapshot.  This is called *graph inference*.  The result is an
ordinary CocoGL resource graph containing a description of all resources, their properties, and their dependencies.

Snapshotting can make adoption of Coconut easier if you already have an environment you wish to model.  It can also
facilitate identifying "drift" between a desired and actual state; we will see more about this in a moment.

Any CocoGL generated in this manner may have less information than CocoGL generated from true CocoLang source code, due
to the possibility of lossy representations and/or missing abstractions in an actual live environment.  For example,
there could be "hidden" implicit dependencies between resources that cannot be discovered and hence will be missing.
Nevertheless, this can be a great first step towards adopting Coconut for your existing environments.

Generating a CocoGL snapshot from a live environment that was created using Coconut, on the other hand, can recover all
of this information reliably, thanks to special tagging that Coconut performs.

Even then, because some objects map to physical resources in a deployment -- like a VM, subnet, etc. -- and other
objects are mere abstractions, there is a limit to how much "reverse engineering" from a live environment can happen.
The creation of an abstraction merely serves to create those physical resources at the "bottom" of the dependency chain.

### Comparing Two CocoGLs

A primary feature of CocoGLs is that two of them can be compared to produce a diff.  This has several use cases.

Coconut performs a diff between two CocoGL files to determine a delta for purposes of incremental deployment.  This
allows it to change the live environment only where a difference between actual and desired state exists.

As seen above, CocoGL snapshot can be generated from a live environment.  As such, a snapshot can be compared to another
graph -- perhaps generated from another live environment -- to determine and reconcile "drift" between them.

This capability can be used to discover differences between environments that are meant to be similar (e.g., in
different zones, staging vs. production, etc).  Alternatively, this analysis could be used to to compare an environment
against a CocoLang program's resulting CocoGL, to identify places where manual changes were made to an actual
environment without having made corresponding changes in the sources (be it intentional or accidental).

To cope with some of the potential lossiness during snapshotting, Coconut implements a *semantic diff*, in addition
to a more strict exact diff, algorithm.  The semantic diff classifies differences due to lossy inference distinct from
ordinary semantically meaningful differences that could be impacting a live environment's behavior.

### Creating or Updating CocoLang and CocoPack from CocoGL

All artifacts embed debugging information similar to [DWARF](https://en.wikipedia.org/wiki/DWARF) or [PDB](
https://en.wikipedia.org/wiki/Program_database), correlating source lines with the resulting artifacts.  As a result,
it is possible to raise CocoGL into CocoPack and, from there, raise CocoPack into your favorite CocoLang.

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

