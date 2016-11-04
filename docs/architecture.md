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

### Translation

Now let us look at how a Mufile turns into a ready-to-run package.  This will not describe the metadata and precise
translation targets (those are available as separate docs [here](metadata.md) and [here](targets.md), respectively);
instead, we will look at the tools, plugin architecture, and overall translation process.

TODO(joe): write this section; just cover the overall structure and plugin model, not the details of each target.

In the future, we envision that Mufiles might be generated from code, for an even more seamless developer experience.
This is very powerful when envisioning serverless architectures, where Stacks, Services, Functions, and Triggers can be
expressed all in a single file, and managed alongside the code and libraries they depend upon.  See marapongo/mu#xxx
for a work item tracking this.  For now, and to simplify this doc, we will ignore this possibility.

### Deployments

TODO(joe): discuss the concept of a deployment.

TODO(joe): describe blue/green zero downtime deployments.

### Command Line Interface

TODO(joe): deployment, ongoing interactions, management, etc.

## System Services

TODO(joe): describe package manager, artifact repository, CI/CD, the relationship between them, etc.

