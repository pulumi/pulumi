CHANGELOG
=========

## HEAD (Unreleased)

- Python SDK fix for a crash resulting from a KeyError if secrets were used in configuration.

- Fix an issue where a secret would not be encrypted in the state file if it was
  a property of a resource which was used as a stack output (fixes
  [#2862](https://github.com/pulumi/pulumi/issues/2862))

## 0.17.20 (2019-06-23)

- SDK fix for crash that could occasionally happen if there were multiple identical aliases to the
  same Resource.

## 0.17.19 (2019-06-23)

- Engine fix for crash that could occasionally happen if there were multiple identical aliases to
  the same Resource.

## 0.17.18 (2019-06-20)

- Allow setting backend URL explicitly in `Pulumi.yaml` file

- `StackReference` now has a `.getOutputSync` function to retrieve exported values from an existing
  stack synchronously.  This can be valuable when creating another stack that wants to base
  flow-control off of the values of an existing stack (i.e. importing the information about all AZs
  and basing logic off of that in a new stack). Note: this only works for importing values from
  Stacks that have not exported `secrets`.

- When the environment variable `PULUMI_TEST_MODE` is set to `true`, the
  Python runtime will now behave as if
  `pulumi.runtime.settings._set_test_mode_enabled(True)` had been called. This
  mirrors the behavior for NodeJS programs (fixes [#2818](https://github.com/pulumi/pulumi/issues/2818)).

- Resources that are only 'read' will no longer be displayed in the terminal tree-display anymore.
  These ended up heavily cluttering the display and often meant that programs without updates still
  showed a bunch of resources that weren't important.  There will still be a message displayed
  indicating that a 'read' has happened to help know that these are going on and that the program is making progress.

## 0.17.17 (2019-06-12)

### Improvements

- docs(login): escape codeblocks, and add object store state instructions
  [#2810](https://github.com/pulumi/pulumi/pull/2810)
- The API for passing along a custom provider to a ComponentResource has been simplified.  You can
  now just say `new SomeComponentResource(name, props, { provider: awsProvider })` instead of `new
  SomeComponentResource(name, props, { providers: { "aws" : awsProvider } })`
- Fix a bug where the path provided to a URL in `pulumi login` is lost are dropped, so if you `pulumi login
  s3://bucketname/afolder`, the Pulumi files will be inside of `s3://bucketname/afolder/.pulumi` rather than
  `s3://bucketname/.pulumi` (thanks, [@bigkraig](https://github.com/bigkraig)!).  **NOTE**: If you have been
  logging in to the s3 backend with a path after the bucket name, you will need to either move the .pulumi
  folder in the bucket to the correct location or log in again without the path prefix to see your previous
  stacks.
- Fix a crash that would happen if you ran `pulumi stack output` against an empty stack (fixes
  [pulumi/pulumi#2792](https://github.com/pulumi/pulumi/issues/2792)).
- Unparented Pulumi `CustomResource`s now support calling `.getProvider(...)` on them.

## 0.17.16 (2019-06-06)

### Improvements

- Fixed a bug that caused an assertion when dealing with unchanged resources across version upgrades.

## 0.17.15 (2019-06-05)

### Improvements

- Pulumi now allows Python programs to "read" existing resources instead of just creating them. This feature enables
  Pulumi Python packages to expose ".get()" methods that allow for reading of resources that already exist.
- Support for referencing the outputs of other Pulumi stacks has been added to the Pulumi Python libraries via the
  `StackReference` type.
- Add CI system detection for Bitbucket Pipelines.
- Pulumi now tolerates changes in default providers in certain cases, which fixes an issue where users would see
  unexpected replaces when upgrading a Pulumi package.
- Add support for renaming resources via the `aliases` resource option.  Adding aliases allows new resources to match
  resources from previous deployments which used different names, maintaining the identity of the resource and avoiding
  replacements or re-creation of the resource.
- `pulumi plugin install` gained a new optional argument `--server` which can be used to provide a custom server to be
  used when downloading a plugin.

## 0.17.14 (2019-05-28)

### Improvements

- `pulumi refresh` now tries to install any missing plugins automatically like
  `pulumi destroy` and `pulumi update` do (fixes [pulumi/pulumi#2669](https://github.com/pulumi/pulumi/issues/2669)).
- `pulumi whoami` now outputs the URL of the currently connected backend.
- Correctly suppress stack outputs when serializing previews to JSON, i.e. `pulumi preview --json --suppress-outputs`.
  Fixes [pulumi/pulumi#2765](https://github.com/pulumi/pulumi/issues/2765).

## 0.17.13 (2019-05-21)

### Improvements

- Fix an issue where creating a first class provider would fail if any of the
  configuration values for the providers were secrets. (fixes [pulumi/pulumi#2741](https://github.com/pulumi/pulumi/issues/2741)).
- Fix an issue where when using `--diff` or looking at details for a proposed
  updated, the CLI might print text like: `<{%reset%}>
  --outputs:--<{%reset%}>` instead of just `--outputs:--`.
- Fixes local login on Windows.  Specifically, windows local paths are properly understood and
  backslashes `\` are not converted to `__5c__` in paths.
- Fix an issue where some operations would fail with `error: could not deserialize deployment: unknown secrets provider type`.
- Fix an issue where pulumi might try to replace existing resources when upgrading to the newest version of some resource providers.

## 0.17.12 (2019-05-15)

### Improvements

- Pulumi now tells you much earlier when the `--secrets-provider` argument to
  `up` `init` or `new` has the wrong value. In addition, supported values are
  now listed in the help text. (fixes [pulumi/pulumi#2727](https://github.com/pulumi/pulumi/issues/2727)).
- Pulumi no longer prompts for your passphrase twice during operations when you
  are using the passphrase based secrets provider. (fixes [pulumi/pulumi#2729](https://github.com/pulumi/pulumi/issues/2729)).
- Fix an issue where complex inputs to a resource which contained secret values
  would not be stored correctly.
- Fix a panic during property diffing when comparing two secret arrays.

## 0.17.11 (2019-05-13)

### Major Changes

#### Secrets and Pluggable Encryption

- The Pulumi engine and Python and NodeJS SDKs now have support for tracking values as "secret" to ensure they are
  encrypted when being persisted in a state file. `[pulumi/pulumi#397](https://github.com/pulumi/pulumi/issues/397)`

  Any existing value may be turned into a secret by calling `pulumi.secret(<value>)` (NodeJS) or
  `Output.secret(<value>`) (Python).  In both cases, the returned value is an output which may be passed around
  like any other.  If this value flows into a resource, the plaintext will not be stored in the state file, but instead
  It will be encrypted, just like values added to config with `pulumi config set --secret`.

  You can verify that values are being stored as you expect by running `pulumi stack export`, When values are encrypted
  in the state file, they appear as an object with a special signature key and a ciphertext property.

  When outputs of a stack are secrets, `pulumi stack output` will show `[secret]` as the value, by default.  You can
  pass `--show-secrets` to `pulumi stack output` in order to see the actual raw value.

- When storing state with the Pulumi Service, you may now elect to use the passphrase based encryption for both secret
  configuration values and values that are encrypted in a state file.  To use this new feature, pass
  `--secrets-provider passphrase` to `pulumi new` or `pulumi stack init` when you initally create the stack. When you
  create the stack, you will be prompted for a passphrase (or if `PULUMI_CONFIG_PASSPHRASE` is set, it will be used).
  This passphrase is used to generate a unique key for your stack, and config values and encrypted state values are
  encrypted using AES-256-GCM. The key is derived from your passphrase, and while information to re-create it when
  provided with your passphrase is stored in both the `Pulumi.<stack-name>.yaml` file and the state file for your stack,
  this information can not be used to recover the key. When using this mode, the Pulumi Service is unable to decrypt
  either your secret configuration values or and secret values in your state file.

  We will be adding gestures to move existing stacks managed by the service to use passphrase based encryption soon
  as well as gestures to change the passphrase for an existing stack.

** Note **

Stacks with encrypted secrets in their state files can only be managed by 0.17.11 or later of the CLI. Attempting
to use a previous version of the CLI with these stacks will result in an error.

Fixes #397

### Improvements

- Add support for Azure Pipelines in CI environment detection.
- Minor fix to how Azure repository information is extracted to allow proper grouping of Azure
  repositories when various remote URLs are used to pull the repository.

## 0.17.10 (2019-05-02)

### Improvements

- Fixes issue introduced in 0.17.9 where local-login broke on Windows due to the new support for
  `s3://`, `azblob://` and `gs://` save locations.
- Minor contributing document improvement.
- Warnings from `npm` about missing description, repository, and license fields in package.json are
  now suppressed when `npm install` is run from `pulumi new` (via `npm install --loglevel=error`).
- Depend on newer version of gRPC package in the NodeJS SDK. This version has
  prebuilt binaries for Node 12, which should make installing `@pulumi/pulumi`
  more reliable when running on Node 12.

## 0.17.9 (2019-04-30)

### Improvements

- `pulumi login` now supports `s3://`, `azblob://` and `gs://` paths (on top of `file://`) for
  storing stack information. These are passed the location of a desired bucket for each respective
  cloud provider (i.e. `pulumi login s3://mybucket`).  Pulumi artifacts (like the
  `xxx.checkpoint.json` file) will then be stored in that bucket.  Credentials for accessing the
  bucket operate in the normal manner for each cloud provider.  i.e. for AWS this can come from the
  environment, or your `.aws/credentials` file, etc.
- The pulumi version update check can be skipped by setting the environment variable
  `PULUMI_SKIP_UPDATE_CHECK` to `1` or `true`.
- Fix an issue where the stack would not be selected when an existing stack is specified when running
  `pulumi new <template> -s <existing-stack>`.
- Add a `--json` flag (`-j` for short) to the `preview` command. This allows basic serialization of a plan,
  including the anticipated set of deployment steps, list of diagnostics messages, and summary information.
  Each step includes deeply serialized information about the resource state and step metadata itself. This
  is part of ongoing work tracked in [pulumi/pulumi#2390](https://github.com/pulumi/pulumi/issues/2390).

## 0.17.8 (2019-04-23)

### Improvements

- Add a new `ignoreChanges` option to resource options to allow specifying a list of properties to
  ignore for purposes of updates or replacements.  [#2657](https://github.com/pulumi/pulumi/pull/2657)
- Fix an engine bug that could lead to incorrect interpretation of the previous state of a resource leading to
  unexpected Update, Replace or Delete operations being scheduled. [#2650]https://github.com/pulumi/pulumi/issues/2650)
- Build/push `pulumi/actions` container to [DockerHub](https://hub.docker.com/r/pulumi/actions) with new SDK releases [#2646](https://github.com/pulumi/pulumi/pull/2646)

## 0.17.7 (2019-04-17)

### Improvements

- A new "test mode" can be enabled by setting the `PULUMI_TEST_MODE` environment variable to
  `true` in either the Node.js or Python SDK. This new mode allows you to unit test your Pulumi programs
  using standard test harnesses, without needing to run the program using the Pulumi CLI. In this mode, limited
  functionality is available, however basic resource object allocation with input properties will work.
  Note that no actual engine operations will occur in this mode, and that you'll need to use the
  `PULUMI_CONFIG`, `PULUMI_NODEJS_PROJECT`, and `PULUMI_NODEJS_STACK` environment variables to control settings
  the CLI would have otherwise managed for you.

## 0.17.6 (2019-04-11)

### Improvements

- `refresh` will now warn instead of returning an error when it notices a resource is in an
  unhealthy state. This is in service of https://github.com/pulumi/pulumi/issues/2633.

## 0.17.5 (2019-04-08)

### Improvements
- Correctly handle the case where we would fail to detect an archive type if the filename included a dot in it. (fixes [pulumi/pulumi#2589](https://github.com/pulumi/pulumi/issues/2589))
- Make `Config`'s constructor's `name` argument optional in Python, for consistency with our Node.js SDK. If it isn't
    supplied, the current project name is used as the default.
- `pulumi logs` will now display log messages from Google Cloud Functions.

## 0.17.4 (2019-03-26)

### Improvements

- Don't print the `error:` prefix when Pulumi exists because of a declined confirmation prompt (fixes [pulumi/pulumi#458](https://github.com/pulumi/pulumi/issues/2070))
- Fix issue where `Outputs` produced by `pulumi.interpolate` might have values which could
  cause validation errors due to them containing the text `<computed>` during previews.

## 0.17.3 (2019-03-26)

### Improvements

- A new command, `pulumi stack rename` was added. This allows you to change the name of an existing stack in a project. Note: When a stack is renamed, the `pulumi.getStack` function in the SDK will now return a new value. If a stack name is used as part of a resource name, the next `pulumi up` will not understand that the old and new resources are logically the same. We plan to support adding aliases to individual resources so you can handle these cases. See [pulumi/pulumi#458](https://github.com/pulumi/pulumi/issues/458) for discussion on this new feature. For now, if you are unwilling to have `pulumi up` create and destroy these resources, you can rename your stack back to the old name. (fixes [pulumi/pulumi#2402](https://github.com/pulumi/pulumi/issues/2402))
- Fix two warnings that were printed when using a dynamic provider about missing method handlers.
- A bug in the previous version of the Pulumi CLI occasionally caused the Pulumi Engine to load the incorrect resource
  plugin when processing an update. This bug has been fixed in 0.17.3 by performing a deterministic selection of the
  best set of plugins available to the engine before starting up. See
- Add support for serializing JavaScript function that capture [BigInts](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/BigInt).
- Support serializing arrow-functions with deconstructed parameters.

## 0.17.2 (2019-03-15)

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

## 0.17.1 (2019-03-06)

### Improvements

- Slight tweak to `Output.apply` signature to help TypeScript infer types better.

## 0.17.0 (2019-03-05)

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


## 0.16.19 (2019-03-04)

- Rolled back change where calling toString/toJSON on an Output would cause a message
  to be logged to the `pulumi` diagnostics stream.

## 0.16.18 (2019-03-01)

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

## 0.16.17 (2019-02-27)

### Improvements

- Rolling back the change:
    "Depending on a Resource will now depend on all other Resource's parented by that Resource."

  Unforeseen problems cropped up that caused deadlocks.  Removing this change until we can
  have a high quality solution without these issues.

## 0.16.16 (2019-02-24)

### Improvements

- Fix deadlock with resource dependencies (https://github.com/pulumi/pulumi/issues/2470)

## 0.16.15 (2019-02-22)

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

## 0.16.14 (2019-01-31)

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

## 0.16.13 (2019-01-31)

### Major Changes

- When used in conjunction with the latest versions of the various language SDKs, the Pulumi CLI is now more precise about the dependent resources that must be deleted when a given resource must be deleted before it can be replaced (fixes [pulumi/pulumi#2167](https://github.com/pulumi/pulumi/issues/2167)).

**NOTE**: As part of the above change, once a stack is updated with v0.16.13, previous versions of `pulumi` will be unable to manage it.

### Improvements

- Issue a more prescriptive error when using StackReference and the name of the stack to reference is not of the form `<organization>/<project>/<stack>`.

## 0.16.12 (2019-01-25)

### Major Changes

- When using the cloud backend, stack names now must only be unique within a project, instead of across your entire account. Starting with version of 0.16.12 the CLI, you can create stacks with duplicate names. If an account has multiple stacks with the same name across different projects, you must use 0.16.12 or later of the CLI to manage them.

**BREAKING CHANGE NOTICE**: As part of the above change, when using the 0.16.12 CLI (or a later version) the names passed to `StackReference` must be updated to be of the form (`<organization>/<project>/<stack>`) e.g. `acmecorp/infra/dev` to refer to the `dev` stack of the `infra` project in the `acmecorp` organization.

### Improvements

- Add `--json` to `pulumi config`, `pulumi config get`, `pulumi history` and `pulumi plugin ls` to request the output be in JSON.

- Changes to `pulumi new`'s output to improve the experience.

## 0.16.11 (2019-01-16)

### Improvements

- In the nodejs SDK, `pulumi.interpolate` and `pulumi.concat` have been added as convenient ways to combine Output values into strings.

- Added `pulumi history` to show information about the history of updates to a stack.

- When creating a project with `pulumi new` the generated `Pulumi.yaml` file no longer contains the template section, which was unused after creating a project

- In the Python SDK, the `is_dry_run` function just always returned `true`, even when an update (and not a preview) was being preformed. This has been fixed.

- Python programs will no longer deadlock due to exceptions in functions run during applies.

## 0.16.10 (2019-01-11)

### Improvements

- Support for first-class providers in Python.

- Fix a bug where `StackReference` outputs were not updated when changes occured in the referenced stack.

- Added `pulumi stack tag` commands for managing stack tags stored in the cloud backend.

- Link directly to /account/tokens when prompting for an access token.

- Exporting a Resource from an application Stack now exports it as a rich recursive pojo instead of just being an opaque URN (fixes https://github.com/pulumi/pulumi/issues/1858).

## 0.16.9 (2018-12-24)

### Improvements

- Update the error message when When `pulumi` commands fail to detect your project to mention that `pulumi new` can be used to create a new project (fixes [pulumi/pulumi#2234](https://github.com/pulumi/pulumi/issues/2234))

- Added a `--stack` argument (short form `-s`) to `pulumi stack`, `pulumi stack init`, `pulumi state delete` and `pulumi state unprotect` to allow operating on a different stack than the currently selected stack. This brings these commands in line with the other commands that operate on stacks and already provided a `--stack` option (fixes [pulumi/pulumi#1648](https://github.com/pulumi/pulumi/issues/1648))

- Added `Output.all` and `Output.from_input` to the Python SDK.

- During previews and updates, read operations (i.e. calls to `.get` methods) are no longer shown in the output unless they cause any changes.

- Fix a performance regression where `pulumi preview` and `pulumi up` would hang for a few moments at the end of a preview or update, in addition to the overall operation being slower.

## 0.16.8 (2018-12-14)

### Improvements

- Fix an issue that caused panics due to shutting the Jaeger tracing infrastructure down before all traces had finished ([pulumi/pulumi#1850](https://github.com/pulumi/pulumi/issues/1850))

## 0.16.7 (2018-12-05)

### Improvements

- Configuration and stack commands now take a `--config-file` options. This option allows the user to override the file used to fetch and store config information for a stack during the execution of a command.

- Fix an issue where ANSI escape codes would appear in messages printed from the CLI when running on Windows.

- Fix an error about a bad icotl when trying to read sensitive input from the console and standard in was not connected to a terminal.

- The dynamic provider would fail to launch if your `node_modules` folder was non in the default location or had a non standard layout. This has been fixed so we correctly find your `node_modules` folder in the same way node does. (fixes [pulumi/pulumi#2261](https://github.com/pulumi/pulumi/issues/2261))

## 0.16.6 (2018-11-28)

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

## 0.16.5 (2018-11-16)

### Improvements

- Fix an issue where `pulumi plugin install` would fail on Windows with an access deined message.

## 0.16.4 (2018-11-12)

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

- Pulumi no longer prompts you for confirmation when `--skip-preview` is passed to `pulumi up`. Instead, it just preforms the update as requested.

- Add the `--json` flag to the `pulumi stack ls` command.

- The `--color=always` flag should now be respected in all cases.

- Pulumi now reports metadata about GitLab repositories when doing an update, so they can be shown on app.pulumi.com.

- Pulumi now uses compression when uploading your checkpoint file to the Pulumi service, which should speed up updates where your stack has many resources.

- "First Class" providers used to be shown as changing during previews. This is no longer the case.
