(plugins)=
# Plugins

Plugins are Pulumi's core extensibility mechanism, allowing the Pulumi engine to
communicate in a uniform manner with various languages, resource providers, and
other tools. Generally speaking, plugins are run as separate processes and often
(though not always) communicated with over gRPC. Presently, Pulumi supports the
following kinds of plugins:

* *Resource plugins* (or *resource providers*/*providers*, see
  [providers](providers) for more) expose a [standardized gRPC
  interface](pulumirpc.ResourceProvider) for managing resources (such as those
  in an AWS or GCP cloud).
* [*Language plugins*](languages) host programs written in a particular
  language, allowing the engine to invoke Pulumi programs without having to
  understand the specifics of their implementation.
* *Analyzer plugins* are used to analyze Pulumi programs for potential issues
  before they are executed. Analyzers underpin [CrossGuard, Pulumi's Policy as
  Code](https://www.pulumi.com/docs/using-pulumi/crossguard/) product.
* [*Converter plugins*](converters) support the conversion of existing
  infrastructure as code (e.g.
  [Terraform](https://github.com/pulumi/pulumi-converter-terraform)) to Pulumi
  programs.
* *Tool plugins* allow integrating Pulumi with arbitrary tools.

(plugin-loading-execution)=
(shimless)=
## Loading and execution

Plugins may be provided in one of two ways:

* As a binary that can be directly executed. Binaries are named
  `pulumi-<kind>-<name>`, where `<kind>` is one of `resource`, `language`,
  `analyzer`, `converter`, or `tool`, and `<name>` is a unique name for the
  plugin. For example, the AWS resource provider plugin is named
  `pulumi-resource-aws`.
* As a directory containing a `PulumiPlugin.yaml` file and a set of files that
  implement the plugin's functionality. The `PulumiPlugin.yaml` file specifies
  the `runtime` to be used to execute the provided files. The engine reads this
  and spawns the runtime's [language](language-hosts) plugin to run the plugin.
  The language plugin's [](pulumirpc.LanguageRuntime.RunPlugin) method is then
  used to execute the plugin. This method of running plugins is sometimes
  referred to as *shimless*, since prior to its introduction one would always
  have to provide a "shim" executable (such as a shell script or batch file)
  that did nothing but spawn the relevant interpreter over the provided files.
