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

## Installation

Right now, there is no unified algorithm for resolving and installing a plugin. The only
way to understand how installation works is to look at each location where we install
plugins.

### Where we install plugins

The `pulumi` CLI installs provider plugins in a lot of places:
- Within the engine when a missing package is needed: preview, up, refresh, destroy.
- `pulumi install`
- `pulumi package add`
- `pulumi import`
- `pulumi plugin install`
- `pulumi package get-schema`
- `pulumi package publish`
- During schema binding

#### Engine installs

Engine behavior depends on what is present in the global cache, but only for plugins in
state where the plugin doesn’t specify a version.  Installs do not handle plugins that
themselves have dependencies at all. You can get subtly different behavior between prompt
and lazy installs for packages that are specified in the packages section of a project.

##### Prompt

Engine related installs all call `engine.EnsurePluginsAreInstalled`. For plugins specs
that are passed to that function, we use on disk versions if present, otherwise we fetch
the latest version. After plugins are downloaded, `pkg/workspace.InstallPluginContent` is
called to install the plugin. This implementation is project aware, meaning that it takes
into account what is packages are present in the `packages` section of your `Pulumi.yaml`.

##### Lazy

The engine also installs plugins as needed when a register resource request comes for a
non-downloaded plugin. Here the engine calls `pkg/workspace.InstallPlugin`, which is not
project aware.

#### pulumi install

As of [pr#20945](https://github.com/pulumi/pulumi/pull/20945) (released in [v3.208.0](https://github.com/pulumi/pulumi/releases/tag/v3.208.0)), plugins with local paths correctly have
their dependencies installed before they are installed, but this logic only works for
local paths, it doesn’t work for git based components with dependencies. The root install
function for local packages is `pkg/workspace.InstallPlugin`, but otherwise we use
`engine.EnsurePluginsAreInstalled`.

#### pulumi plugin install

This is the only callsite currently set up to resolve registry packages. Packages are not
installed if there is already a version installed and no version is specified or **the
version specified is < the already installed version**, unless `--exact` is passed. The
install is not project aware.


#### pulumi package add

`pulumi package add` ends up calling `packages.ProviderFromSource`, which calls
`pkg/workspace.InstallPlugin`. That means it does not work on packages that depend on
other packages. This implementation is not project aware.

#### pulumi package get-schema

`pulumi package get-schema` also calls `packages.SchemaFromSchemaSource`, which calls into
`packages.ProviderFromSource`, same as `pulumi package add`.

#### pulumi package publish

Also uses `packages.SchemaFromSchemaSource`, with the same semantics.

#### Schema binding

Schema binding also winds up calling `pkg/workspace.InstallPlugin`. This is not project
aware (and it’s not clear it should be here).

### Correctly installing plugins

When I say install, I mean going from a package descriptor to a running package.  To
install a given package descriptor, we need to:

1. Resolve the package descriptor into a concrete plugin + parameterization
1. Download the plugin (if required)
1. Install any dependent packages (recursively)
1. Generate and link in SDKs for any dependent packages.
1. Install language specific dependencies.

This algorithm is implemented for production in pr#21177.

All of the above steps are fallible, which means that a given plugin can be in any of the
following states:

1. Not present on disk
1. Present on disk
1. Installed

(2) can be because the installation hasn't happened yet, or because the installation
failed for some reason. We distinguish between (2) and (3) by the presence of a marker
file: `<plugin-name>.partial` means present but not installed.
