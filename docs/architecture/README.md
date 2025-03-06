# Architecture

We can approach Pulumi's architecture by breaking down how a user interacts with
it. Pulumi (or "Pulumi IaC", "Pulumi Infrastructure as Code" when
differentiating from other Pulumi products) allows users to manage their
infrastructure using the programming languages and technologies they are already
familiar with:

* Users write *programs* in a *language* of their choice (e.g. TypeScript or
  Python).

* Programs declare a number of *resources* (e.g. virtual machines, or storage
  buckets) that the user wants to manage in various *providers* (e.g. AWS, Azure
  or GCP).

* Programs are written using the *core Pulumi SDK* (e.g. `@pulumi/pulumi` in
  TypeScript) and one or more *provider SDKs* (e.g. `@pulumi/aws` for AWS).

* Users invoke the *Pulumi command-line interface* (CLI) to run their programs
  and invoke particular *operations* (e.g. preview, update, or refresh).
  An instance of a program is called a *stack* (e.g. `development`, or
  `production`). Stacks are grouped into *projects*.

* The CLI creates an instance of the *deployment engine* to manage the lifecycle
  of the operation. The engine interacts with both the program and the set of
  relevant providers in order to achieve a desired *state*.

* When the operation completes, the engine persists its view of the state to a
  configured *storage backend* (e.g. a local file, or a service like Pulumi
  Cloud).

In order to support the various languages and providers that Pulumi offers,
these pieces are composed as a set of *processes* that communicate with each
other using *[gRPC](https://grpc.io/) remote procedure calls*. These calls are
specified as [Protocol Buffer](https://protobuf.dev) interfaces (*Protobufs*).
Excluding language-specific components (e.g. SDKs), the majority of these
processes are implemented as libraries and executables written in
[Go](https://golang.org):

* Programs are executed by a *language host* process, which wraps the runtime
  for a given language in an interface that the deployment engine can interact
  with.

* *Provider processes* (often just "providers") expose a standard interface for
  performing validation, diff, create, read, update, and delete operations on
  resources. Providers typically also support function and method calls on the
  provider and resource objects (known as *invokes* and *calls* respectively).

* The deployment engine orchestrates language hosts and provider processes to
  achieve the desired state of the infrastructure. As well as calling procedures
  on these processes, the engine itself exposes procedures that can be called
  back into by the language hosts and providers. Perhaps the most important of
  these is *resource registration*, whereby a program indicates to the engine
  the desired state of a resource (which the engine will then interface with the
  relevant provider to achieve).

* The Pulumi resource model is specified using *schemata*. Provider processes
  expose functions for returning the set of resources and functions that they
  expose in a schema. Language hosts expose functions for generating code
  (e.g. provider SDKs) from a supplied schema.

With this in mind, this architecture is reflected in this repository as
follows:[^side-node-exhaustive]

[^side-node-exhaustive]: This is not intended to be an exhaustive list of
    directories in the repository, but rather a high-level overview of the most
    important ones that pertain to the pieces outlined above. Other concerns,
    such as infrastructure, documentation, testing, and so on, are covered in
    other parts of this documentation.

* `proto` -- contains Protobuf definitions for the various services and messages
  that Pulumi uses to communicate between processes. Code generated from these
  is committed to the repository and used by various components.

* `pkg/codegen` -- contains code generators for the various languages that Pulumi
  supports. As well as supporting language SDK generation as mentioned above
  (*"SDKgen"*), code generators also underpin the `pulumi convert` command, which
  can convert existing infrastructure-as-code (IaC) programs written in e.g.
  Terraform or CloudFormation to Pulumi programs (*"Programgen"*).

* `sdk/{go,nodejs,python,...}` -- contains the core Pulumi SDKs for the various
  languages that Pulumi supports. These SDKs provide the core functionality for
  writing Pulumi programs in a given language, exposing the Pulumi engine's gRPC
  interface (e.g. `RegisterResource`) in an idiomatic way for that language
  (e.g. `new Resource(...)` in TypeScript, or `Resource(...)` in Python).

  The language host for a given language (written in Go) is also typically found
  under that language's SDK directory, and named `pulumi-language-<language>`
  (e.g. `pulumi-language-nodejs` in the case of NodeJS/TypeScript).

* `pkg/{backend,engine,graph,operations,resource}` -- contains the core
  components of the Pulumi engine. The various directories roughly segregate
  some of the responsibilities that fall to the engine, such as:

  * Persisting state to a configured `backend`;
  * Computing and respecting dependency `graph`s so that operations can be
    executed in the correct order;
  * Managing `resource` lifecycles and deployments.

  Of particular note is `pkg/resource/deploy`, which houses a number of the
  pieces required to enact a complete deployment.

* `pkg/{cmd,display}` -- contains code pertinent to the Pulumi CLI. In
  particular, `pulumi/cmd/pulumi` is the entry point to the CLI and exposes the
  various `pulumi preview`, `pulumi up`, `pulumi destroy`, etc. commands that
  you may be familiar with, routing these commands to an instance of the engine
  that the CLI creates and manages.

* With the exception of a number of test providers that live in the core
  `pulumi/pulumi` repository, provider implementations are typically housed in
  their own repositories (e.g. `pulumi-aws` for AWS). These repositories follow
  a similar pattern when it comes to several of the core components -- e.g. most
  providers will have an `sdk/` directory exposing provider SDKs for each
  supported language.

The remainder of this section covers many of these components in more detail, as
well as discussing how they are verified and tested.

:::{toctree}
:maxdepth: 1
:titlesonly:

/docs/architecture/deployment-execution/README
/docs/architecture/types/README
/docs/architecture/plugins
/docs/architecture/providers/README
/docs/architecture/languages
/docs/architecture/converters
/pkg/codegen/README
/docs/architecture/testing/README
:::
