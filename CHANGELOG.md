## 0.17.3 (Unreleased)

### Improvements

- A new command, `pulumi stack rename` was added. This allows you to change the name of an existing stack in a project. Note: When a stack is renamed, the `pulumi.getStack` function in the SDK will now return a new value. If a stack name is used as part of a resource name, the next `pulumi update` will not understand that the old and new resources are logically the same. We plan to support adding aliases to individual resources so you can handle these cases. See [pulumi/pulumi#458](https://github.com/pulumi/pulumi/issues/458) for discussion on this new feature. For now, if you are unwilling to have `pulumi update` create and destroy these resources, you can rename your stack back to the old name. (fixes [pulumi/pulumi#2402](https://github.com/pulumi/pulumi/issues/2402))

## 0.17.2 (Released March 15, 2019)

### Improvements

- Show `brew upgrade pulumi` as the upgrade message when the currently running `pulumi` executable
  is running on macOS from the brew install directory.
- Resource diffs that are rendered to the console are now filtered to properties that have semantically-meaningful
  changes where possible.
- `pulumi new` no longer runs an initial deployment after a project is generated for nodejs projects.
  Instead, instructions are printed indicating that `pulumi up` can be used to deploy the project.
- Differences between the state of a refreshed resource and the state described in a Pulumi program are now properly
  detected when using newer providers.
- Differences between a resource's provider-internal properties are no longer displayed in the CLI.
- Pulumi will now install missing plugins on startup. Previously, Pulumi would error if a required plugin was not
  present and a bug in the Pulumi CLI made it common for users using Pulumi in their continuous integration setup to have problems with missing plugins. Starting with 0.17.2, if Pulumi detects that required plugins are missing, it will make an attempt to install the missing plugins before proceeding with the update.

## 0.17.1 (Released March 6, 2019)

### Improvements

- Slight tweak to `Output.apply` signature to help TypeScript infer types better.

## 0.17.0 (Released March 5, 2019)

This update includes several changes to core `@pulumi/pulumi` constructs that will not play nicely
in side-by-side applications that pull in prior versions of this package.  As such, we are rev'ing
the minor version of the package from 0.16 to 0.17.  Recent version of `pulumi` will now detect,
and warn, if different versions of `@pulumi/pulumi` are loaded into the same application.  If you
encounter this warning, it is recommended you move to versions of the `@pulumi/...` packages that
are compatible.  i.e. keep everything on 0.16.x until you are ready to move everything to 0.17.x.

### Improvements

- `Output<T>` now 'lifts' property members from the value it wraps, simplifying common coding patterns.
For example:

```ts
interface Widget { text: string, x: number, y: number };
var v: Output<Widget>;

var widgetX = v.x;
// `widgetX` has the type Output<number>.
// This is equivalent to writing `v.apply(w => w.x)`
```

Note: this 'lifting' only occurs for POJO values.  It does not happen for `Output<Resource>`s.
Similarly, this only happens for properties.  Functions are not lifted.

- Depending on a **Component** Resource will now depend on all other Resources parented by that
  Resource. This will help out the programming model for Component Resources as your consumers can
  just depend on a Component and have that automatically depend on all the child Resources created
  by that Component.  Note: this does not apply to a **Custom** resource.  Depending on a
  CustomResource will still only wait on that single resource being created, not any other Resources
  that consider that CustomResource to be a parent.


## 0.16.19 (Released March 4, 2019)

- Rolled back change where calling toString/toJSON on an Output would cause a message
  to be logged to the `pulumi` diagnostics stream.

## 0.16.18 (Released March 1, 2019)

- Fix an issue where the Pulumi CLI would load the newest plugin for a resource provider instead of the version that was
  requested, which could result in the Pulumi CLI loading a resource provider plugin that is incompatible with the
  program. This has the potential to disrupt users that previously had working configurations; if you are experiencing
  problems after upgrading to 0.16.17, you can opt-in to the legacy plugin load behavior by setting the environnment
  variable `PULUMI_ENABLE_LEGACY_PLUGIN_SEARCH=1`. You can also install plugins that are missing with the command
  `pulumi plugin install resource <name> <version> --exact`.

### Improvements

- Attempting to convert an [Output<T>] to a string or to JSON will now result in a warning
  message being printed, as well as information on how to rectify the situation.  This is
  to help with diagnosing cryptic problems that can occur when Outputs are accidentally
  concatenated into a string in some part of the program.

- Fixes incorrect closure serialization issue (https://github.com/pulumi/pulumi/pull/2497)

- `pulumi` will now check that all versions of `@pulumi/pulumi` are compatible in your node_modules
  folder, and will issue a warning message if not.  To be compatible, the versions of
  `@pulumi/pulumi` must agree on their major and minor versions.  Running incompatible versions is
  not something that will be blocked, but it is discouraged as it may lead to subtle problems if one
  version of `@pulumi/pulumi` is loaded and passes objects to/from an incompatible version.

## 0.16.17 (Released February 27th, 2019)

### Improvements

- Rolling back the change:
    "Depending on a Resource will now depend on all other Resource's parented by that Resource."

  Unforseen problems cropped up that caused deadlocks.  Removing this change until we can
  have a high quality solution without these issues.

## 0.16.16 (Released February 24th, 2019)

### Improvements

- Fix deadlock with resource dependencies (https://github.com/pulumi/pulumi/issues/2470)

## 0.16.15 (Released February 22nd, 2019)

### Improvements

- When trying to `stack rm` a stack managed by pulumi.com that has resources, the error message now informs you to pass `--force` if you really want to remove a stack that still has resources under management, as this would orphan these resources (fixes [pulumi/pulumi#2431](https://github.com/pulumi/pulumi/issues/2431)).
- Enabled Python programs to delete resources in parallel (fixes [pulumi/pulumi#2382](https://github.com/pulumi/pulumi/issues/2382)). If you are using Python 2, you should upgrade to Python 3 or else you may experience problems when deleting resources.
- Fixed an issue where Python programs would occasionally fail during preview with errors about empty IDs being passed
  to resources. ([pulumi/pulumi#2450](https://github.com/pulumi/pulumi/issues/2450))
- Return an error from `pulumi stack tag` commands when using the `--local` mode.
- Depending on a Resource will now depend on all other Resource's parented by that Resource.
  This will help out the programming model for Component Resources as your consumers can just
  depend on a Component and have that automatically depend on all the child Resources created
  by that Component.

## 0.16.14 (Released January 31st, 2019)

### Improvements

- Fix a regression in `@pulumi/pulumi` introduced by 0.16.13 where an update could fail with an error like:

```
Diagnostics:
  pulumi:pulumi:Stack (my-great-stack):
    TypeError: resproto.InvokeRequest is not a constructor
        at Object.<anonymous> (.../node_modules/@pulumi/pulumi/runtime/invoke.js:58:25)
        at Generator.next (<anonymous>)
        at fulfilled (.../node_modules/@pulumi/pulumi/runtime/invoke.js:17:58)
        at <anonymous>
```

We appologize for the regression.  (fixes [pulumi/pulumi#2414](https://github.com/pulumi/pulumi/issues/2414))

### Improvements

- Individual resources may now be explicitly marked as requiring delete-before-replace behavior. This can be used e.g. to handle explicitly-named resources that may not be able to be replaced in the usual manner.

## 0.16.13 (Released January 31st, 2019)

### Major Changes

- When used in conjuction with the latest versions of the various language SDKs, the Pulumi CLI is now more precise about the dependent resources that must be deleted when a given resource must be deleted before it can be replaced (fixes [pulumi/pulumi#2167](https://github.com/pulumi/pulumi/issues/2167)).

**NOTE**: As part of the above change, once a stack is updated with v0.16.13, previous versions of `pulumi` will be unable to manage it.

### Improvements

- Issue a more prescriptive error when using StackReference and the name of the stack to reference is not of the form `<organization>/<project>/<stack>`.

## 0.16.12 (Released January 25th, 2019)

### Major Changes

- When using the cloud backend, stack names now must only be unique within a project, instead of across your entire account. Starting with version of 0.16.12 the CLI, you can create stacks with duplicate names. If an account has multiple stacks with the same name across different projects, you must use 0.16.12 or later of the CLI to manage them.

**BREAKING CHANGE NOTICE**: As part of the above change, when using the 0.16.12 CLI (or a later version) the names passed to `StackReference` must be updated to be of the form (`<organization>/<project>/<stack>`) e.g. `acmecorp/infra/dev` to refer to the `dev` stack of the `infra` project in the `acmecorp` organization.

### Improvements

- Add `--json` to `pulumi config`, `pulumi config get`, `pulumi history` and `pulumi plugin ls` to request the output be in JSON.

- Changes to `pulumi new`'s output to improve the experience.

## 0.16.11 (Released January 16th, 2019)

### Improvements

- In the nodejs SDK, `pulumi.interpolate` and `pulumi.concat` have been added as convenient ways to combine Output values into strings.

- Added `pulumi history` to show information about the history of updates to a stack.

- When creating a project with `pulumi new` the generated `Pulumi.yaml` file no longer contains the template section, which was unused after creating a project

- In the Python SDK, the `is_dry_run` function just always returned `true`, even when an update (and not a preview) was being preformed. This has been fixed.

- Python programs will no longer deadlock due to exceptions in functions run during applies.

## 0.16.10 (Released January 11th, 2019)

### Improvements

- Support for first-class providers in Python.

- Fix a bug where `StackReference` outputs were not updated when changes occured in the referenced stack.

- Added `pulumi stack tag` commands for managing stack tags stored in the cloud backend.

- Link directly to /account/tokens when prompting for an access token.

- Exporting a Resource from an application Stack now exports it as a rich recursive pojo instead of just being an opaque URN (fixes https://github.com/pulumi/pulumi/issues/1858).

## 0.16.9 (Released December 24th, 2018)

### Improvements

- Update the error message when When `pulumi` commands fail to detect your project to mention that `pulumi new` can be used to create a new project (fixes [pulumi/pulumi#2234](https://github.com/pulumi/pulumi/issues/2234))

- Added a `--stack` argument (short form `-s`) to `pulumi stack`, `pulumi stack init`, `pulumi state delete` and `pulumi state unprotect` to allow operating on a different stack than the currently selected stack. This brings these commands in line with the other commands that operate on stacks and already provided a `--stack` option (fixes [pulumi/pulumi#1648](https://github.com/pulumi/pulumi/issues/1648))

- Added `Output.all` and `Output.from_input` to the Python SDK.

- During previews and updates, read operations (i.e. calls to `.get` methods) are no longer shown in the output unless they cause any changes.

- Fix a performance regression where `pulumi preview` and `pulumi update` would hang for a few moments at the end of a preview or update, in additon to the overall operation being slower.

## 0.16.8 (Released December 14th, 2018)

### Improvements

- Fix an issue that caused panics due to shutting the Jaeger tracing infrastructure down before all traces had finished ([pulumi/pulumi#1850](https://github.com/pulumi/pulumi/issues/1850))

## 0.16.7 (Released December 5th, 2018)

### Improvements

- Configuration and stack commands now take a `--config-file` options. This option allows the user to override the file used to fetch and store config information for a stack during the execution of a command.

- Fix an issue where ANSI escape codes would appear in messages printed from the CLI when running on Windows.

- Fix an error about a bad icotl when trying to read sensitive input from the console and standard in was not connected to a terminal.

- The dynamic provider would fail to launch if your `node_modules` folder was non in the default location or had a non standard layout. This has been fixed so we correctly find your `node_modules` folder in the same way node does. (fixes [pulumi/pulumi#2261](https://github.com/pulumi/pulumi/issues/2261))

## 0.16.6 (Released November 28th, 2018)

### Major Changes

- When running a Python program, pulumi will now run `python3` instead of `python`, since `python` often points at Python 2.7 binary, and Pulumi requires Python 3.6 or later. The environment variable `PULUMI_PYTHON_CMD` can be used to provide a different binary to run.

### Improvements

- Allow `Output`s in the dependsOn property of `ResourceOptions` (fixes [pulumi/pulumi#991](https://github.com/pulumi/pulumi/issues/991))

- Add a new `StackReference` type to the node SDK which allows referencing an output of another stack (fixes [pulumi/pulumi#109](https://github.com/pulumi/pulumi/issues/109))

- Fix an issue where `pulumi` would not respect common `NO_PROXY` settings (fixes [pulumi/pulumi#2134](https://github.com/pulumi/pulumi/issues/2134))

- The CLI wil now correctly report any output from a Python program which writes to `sys.stderr` (fixes [pulumi/pulumi#1542](https://github.com/pulumi/pulumi/issues/1542))

- Don't install packages by default for Python projects when creating a new project from a template using `pulumi new`. Previously, `pulumi` would install these packages using `pip install` and they would be installed globally when `pulumi` was run outside a virtualenv.

- Fix an issue where `pulumi` could panic during a peview when using a first class provider which was constructed using an output property of another resource (fixes [pulumi/pulumi#2223](https://github.com/pulumi/pulumi/issues/2223))

- Fix an issue where `pulumi` would fail to load resource plugins for newer dev builds.

- Fix an issue where running two copies of `pulumi plugin install` in parallel for the same plugin version could cause one to fail with an error about renaming a directory.

- Fix an issue where if the directory containing the `pulumi` executable was not on the `$PATH` we would fail to load language plugins. We now will also search next to the current running copy of Pulumi (fixes [pulumi/pulumi#1956](https://github.com/pulumi/pulumi/issues/1956))

- Fix an issue where passing a key of the form `foo:config:bar:baz` to `pulumi config set` would succeed but cause errors later when trying to interact with the stack. Setting this value is now blocked eagerly (fixes [pulumi/pulumi#2171](https://github.com/pulumi/pulumi/issues/2171))

## 0.16.5 (Released November 16th, 2018)

### Improvements

- Fix an issue where `pulumi plugin install` would fail on Windows with an access deined message.

## 0.16.4 (Released November 12th, 2018)

### Major Changes

- If you're using Pulumi with Python, this release removes Python 2.7 support in favor of Python 3.6 and greater. In addition, some members have been renamed. For example the `stack_output` function has been renamed to `export`. All major features of Pulumi work with this release, including parallelism!

### Improvements

- Download plugins to a temporary folder during `pulumi plugin install` to ensure if the operation is canceled, the have downloaded plugin is not used.

- If an update is in progress when `pulumi stack ls` is run, don't show its last update time as "a long time ago".

- Add `--preserve-config` to `pulumi stack rm` which causes Pulumi to keep the `Pulumi.<stack-name>.yaml` when removing a stack.

- Support passing template names to `pulumi up` the same as `pulumi new` does.

- When `-g` or `--generate-only` is passed to `pulumi new`, don't show a confusing message that says it will update a stack.

- Fix an issue where an output property of a resource would change its type during an update in some cases.

- Provide richer detail on the properties during a multi-stage replace.

- Fix `pulumi logs` so it can collect log messages from Lambdas on AWS.

- Pulumi now reports metadata during CI runs on CircleCI, for later display on app.pulumi.com.

- Fix an assert that could fire if a checkpoint had multiple resources with the same URN (which could happen in cases where a delete operation was pending on an old copy of a resource).

- When `$TERM` is set to `dumb`, Pulumi should no longer try to use interactive reading from the terminal, which would fail.

- When displaying elapsed time for an update, round to the nearest second.

- Add the `--json` flag to the `pulumi logs` command.

- Add an `iterable` module to `@pulumi/pulumi` with two helpful combinators `toObject` and `groupBy` to help combine multiple `Output<T>`'s into a single object.

- Pulumi no longer prompts you for confirmation when `--skip-preview` is passed to `pulumi update`. Instead, it just preforms the update as requested.

- Add the `--json` flag to the `pulumi stack ls` command.

- The `--color=always` flag should now be respected in all cases.

- Pulumi now reports metadata about GitLab repositories when doing an update, so they can be shown on app.pulumi.com.

- Pulumi now uses compression when uploading your checkpoint file to the Pulumi service, which should speed up updates where your stack has many resources.

- "First Class" providers used to be shown as changing during previews. This is no longer the case.
