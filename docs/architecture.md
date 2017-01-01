# Mu Architecture

Mu introduces abstractions that help developers create, maintain, and reason about microservice-based architectures.
More than that, Mu facilitates sharing and reuse of these architectures between developers and teams.

This document describes the overall architecture for the system, including this translation process.

## Concepts

The core top-level architectural abstractions in Mu are:

* **Stack**: A static blueprint describing some specific topology of cloud resources.
* **Service**: An instantiation of a Stack, grouping zero to many services, each with an optional API, together.
* **Cluster**: A dynamic collection of zero to many Stacks deployed together into a shared hosting environment.
* **Workspace**: A static collection of zero to many Stacks managed together in a single source repository.

In an analogy with object-oriented systems, a Stack is akin to a class and Service is akin to an object.

Although a Cluster and Workspace both can contain many Stacks (dynamically and statically, respectively), there isn't
necessarily a one-to-one mapping between them.  Of course, management from the Marapongo console is easier if there is.

Many concepts that are "distinct" in other systems, like the notion of Gateways, Controllers, Functions, Triggers, and
so on, are expressed as Stacks and Services in the Mu system.  They are essentially "subclasses" -- or specializations
-- of this more general concept, unifying the configuration, provisioning, discovery, and overall management of them.

In addition to those core abstractions, there are some supporting ones:

* **Type**: A schematized type, sometimes Stack-based, that is used for type-checking Mu specifications.
* **Identity**: A unit of authentication and authorization, governing access to protected resources.
* **Configuration**: A bag of key/value settings used either at build or runtime.
* **Secret**: A special kind of key/value configuration bag that is encrypted and protected by identity.

Because Mu is a tool for interacting with existing clouds -- including targets like AWS, Kubernetes, and Docker Swarm --
one of the toolchain's most important jobs is faithfully mapping these abstractions onto "lower level" infrastructure
abstractions, and vice versa.  Much of Mu's ability to deliver on its promise of better productivity, sharing, and reuse
relies on its ability to robustly and intuitively perform these translations.

## Toolchain

In this section, we will look at the toolchain that powers the overall Mu architecture.

### Compilation

All descriptions of Mu objects must be "compiled" in order to turn them into runnable artifacts in the target
environment.  This process, like an ordinary compiler, takes some inputs, parses them into an AST that is analyzed for
correctness, and, provided this process occurs error-free, produces some outputs.

This will not describe the metadata and precise translation targets (those are available as separate docs [here](
metadata.md) and [here](targets.md), respectively); instead, we will look at the tools, plugin architecture, and overall
translation process.

Mu inputs may include the following:

* Mufile (`Mu.yaml`): each instance of such a file describes a single outer Stack.
* DSL snippet: a Mufile may execute "code as infrastructure" that produce pieces of the Stack.
* Deployment assets: a Mufile often references assets, like binary program files, that must get deployed.
* Clusterfile (`Mucluster.yaml`): each instance of such a file describes one or more Cluster environments.

The collection of inputs effectively describe a desired state.  In many use-cases, therefore, the desired "output" of
the compilation process is not an artifact at all, but rather a series of actions that accomplish this desired state.

The phases inside of the compilation process may produce intermediate outputs.  For example, if we are targeting AWS,
perhaps a collection of CloudFormation templates are produced before applying to the target environment.  This document
describes each such intermediate output because they can be useful for certain scenarios, although in the common case, a
developer can safely ignore their existence.  It's entirely possible, however, to run the Mu toolchain in a mode where
the backend operations are done outside of the purview of Mu.  For instance, maybe a developer describes everything in
Mufiles, etc., however then hands off the process to IT who edits and applies the CloudFormation outputs manually.

At a high-level, the compilation process look like this:

* Front-end:
    - Parsing: inputs in the form of Mu.yaml and Mucluster.yaml are turned into ASTs.
    - Generation: execution of any "code as infrastructure" artifacts necessary to generate additional input.
    - Expansion: expansion of templates in the artifacts, leveraging configuration and other inputs.
* Middle-end:
    - Semantic analysis: analysis of the results, post generation and expansion, to ensure they are valid.
* Back-end:
    - Targeting: lowering from the AST form to the cloud target's specific representation.
    - Changeset generation: delta analysis to ensure that only changed parts of the topology are modified if possible.
    - Deployment: execution of the resulting changes necessary for the target to reach the desired state.

TODO(joe): describe each of these in more detail.

### Workspaces

A workspace is a root directory on the filesystem that helps to organize many stacks, shared settings among them (like
shared cluster definitions, dependencies, etc).  The root of a workspace is identified by the presence of a
`Muspace.yaml` (or `.json`) file, containing all of the relevant metadata.  The workspace metadata goes here because
stack definitions are generally agnostic to it.  In addition to this, there will be an optional `.Mudeps` directory
that contains all of the downloaded dependencies.

For example, let's say we have two Stacks, `db` and `webapp`; a reasonable workspace structure might be:

    .Mudeps/
        ...
    db/
        Mu.yaml
    webapp/
        Mu.yaml
    Muspace.yaml

For convenience, the home directory `~` can also be its own workspace with identical structure, for settings and
dependencies that are shared by all other workspaces on the machine.

Please refer to [this doc](deps.md) for more information about how dependencies are dealt with and resolved.

### Deployments

TODO(joe): discuss the concept of a deployment.

TODO(joe): describe blue/green zero downtime deployments.

### Command Line Interface

    mu check
    mu diff
    mu build

TODO(joe): deployment, ongoing interactions, management, etc.

## System Services

TODO(joe): describe package manager, artifact repository, CI/CD, the relationship between them, etc.

## Runtime Services

TODO(joe): describe logging, Mu correlation IDs, etc.

TODO(joe): describe what out of the box runtime features are there.  It would be nice if we can do "auto-retry" for
    service connections...somehow.  E.g., see https://convox.com/guide/databases/.

