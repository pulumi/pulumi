# Mu Architecture

Mu introduces abstractions that help developers create, maintain, and reason about microservice-based architectures.
More than that, Mu facilitates sharing and reuse of these architectures between developers and teams.

This document describes the overall architecture for the system, including this translation process.

## Concepts

The three top-level architectural abstractions in Mu are:

* **Stack**: A static blueprint, or "type", that describes a topology of cloud resources.
* **Service**: An instantiation of a Stack, grouping zero to many services, each with an optional API, together.
* **Workspace**: A static collection of one or many Stacks managed together in a single source repository.

In an analogy with object-oriented systems, a Stack is akin to a class and Service is akin to an object.

Many concepts that are "distinct" in other systems, like the notion of Gateways, Controllers, Functions, Triggers, and
so on, are expressed as Stacks and Services in the Mu system.  They are essentially "subclasses" -- or specializations
-- of this more general concept, unifying the configuration, provisioning, discovery, and overall management of them.

Because Mu is a tool for interacting with existing clouds -- including targets like AWS, Kubernetes, and Docker Swarm --
one of the toolchain's most important jobs is faithfully mapping these abstractions onto "lower level" infrastructure
abstractions, and vice versa.  Much of Mu's ability to deliver on its promise of better productivity, sharing, and reuse
relies on its ability to robustly and intuitively perform these translations.

## Toolchain

In this section, we will look at the toolchain that powers the overall Mu architecture.

### Translation

Now let us look at how a Mufile turns into a ready-to-run package.  This will not describe the precise translation and
set of targets (that is available in the [metadata specification document](metadata.md)), instead focusing on the
various bits of data and code involved, the plugin architecture, and overall translation process flow.

TODO(joe): write this section; just cover the overall structure and plugin model, not the details of each target.

In the future, we envision that Mufiles might be generated from code, for an even more seamless developer experience.
This is very powerful when envisioning serverless architectures, where Stacks, Services, Functions, and Triggers can be
expressed all in a single file, and managed alongside the code and libraries they depend upon.  See marapongo/mu#xxx
for a work item tracking this.  For now, and to simplify this doc, we will ignore this possibility.

### Command Line Interface

TODO(joe): deployment, ongoing interactions, management, etc.

## Services

TODO(joe): describe package manager, artifact repository, the relationship between them, etc.

