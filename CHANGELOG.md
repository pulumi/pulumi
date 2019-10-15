CHANGELOG
=========

## HEAD (Unreleased)

- Fix hangs and crashes related to use of `getResource` (i.e. `aws.ec2.getSubnetIds(...)`) methods,
  including frequent hangs on Node.js 12. This fixes https://github.com/pulumi/pulumi/issues/3260)
  and [hangs](https://github.com/pulumi/pulumi/issues/3309).

  Some less common existing styles of using `getResource` calls are also deprecated as part of this
  change, and users should see https://www.pulumi.com/docs/troubleshooting/#synchronous-call for
  details on adjusting their code if needed.

## 1.3.1 (2019-10-09)

- Revert "propagate resource inputs to resource state during preview". These changes had a critical issue that needs
  further investigation.

## 1.3.0 (2019-10-09)

- Propagate resource inputs to resource state during preview, including first-class unknown values. This allows the
  preview to better estimate the state of a resource after an update, including property values that were populated
  using defaults calculated by the provider.
  [#3245](https://github.com/pulumi/pulumi/pull/3245)

- Fetch version information from the Homebrew JSON API for CLIs installed using `brew`.
  [#3290](https://github.com/pulumi/pulumi/pull/3290)

- Support renaming stack projects via `pulumi stack rename`.
  [#3292](https://github.com/pulumi/pulumi/pull/3292)

- Make the location of `.pulumi` folder configurable with an environment variable.
  [#3300](https://github.com/pulumi/pulumi/pull/3300) (Fixes [#2966](https://github.com/pulumi/pulumi/issues/2966))

- `pulumi update` can now be scoped to update a single resource by adding a `--target urn` or `-t urn`
  argument.  Multiple resources can be specified using `-t urn1 -t urn2`.

- Adds the ability to provide transformations to modify the properties and resource options that
  will be used for any child resource of a component or stack.
  [#3174](https://github.com/pulumi/pulumi/pull/3174)

- Add resource transformations support in Python. [#3319](https://github.com/pulumi/pulumi/pull/3319)

## 1.2.0 (2019-09-26)

- Support emitting high-level execution trace data to a file and add a debug-only command to view trace data.
  [#3238](https://github.com/pulumi/pulumi/pull/3238)

- Fix parsing of GitLab urls with subgroups.
  [#3239](https://github.com/pulumi/pulumi/pull/3239)

- `pulumi refresh` can now be scoped to refresh a subset of resources by adding a `--target urn` or
  `-t urn` argument.  Multiple resources can be specified using `-t urn1 -t urn2`.

- `pulumi destroy` can now be scoped to delete a single resource (and its dependents) by adding a
  `--target urn` or `-t urn` argument.  Multiple resources can be specified using `-t urn1 -t urn2`.

- Avoid re-encrypting secret values on each checkpoint write. These changes should improve update times for stacks
  that contain secret values.
  [#3183](https://github.com/pulumi/pulumi/pull/3183)

- Add Codefresh CI detection.

- Add `-c` (config array) flag to the `preview` command.

## 1.1.0 (2019-09-11)

- Fix a bug that caused the Python runtime to ignore unhandled exceptions and erroneously report that a Pulumi program executed successfully.
  [#3170](https://github.com/pulumi/pulumi/pull/3170)

- Read operations are no longer considered changes for the purposes of `--expect-no-changes`.
  [#3197](https://github.com/pulumi/pulumi/pull/3197)

- Increase the MaxCallRecvMsgSize for interacting with the gRPC server.
  [#3201](https://github.com/pulumi/pulumi/pull/3201)

- Do not ask for a passphrase in non-interactive sessions (fix [#2758](https://github.com/pulumi/pulumi/issues/2758)).
  [#3204](https://github.com/pulumi/pulumi/pull/3204)

- Support combining the filestate backend (local or remote storage) with the cloud-backed secrets providers (KMS, etc.).
  [#3198](https://github.com/pulumi/pulumi/pull/3198)

- Moved `@pulumi/pulumi` to target `es2016` instead of `es6`.  As `@pulumi/pulumi` programs run
  inside Nodejs, this should not change anything externally as Nodejs already provides es2016
  support. Internally, this makes more APIs available for `@pulumi/pulumi` to use in its implementation.

- Fix the --stack option of the `pulumi new` command.
  ([#3131](https://github.com/pulumi/pulumi/pull/3131) fixes [#2880](https://github.com/pulumi/pulumi/issues/2880))

## 1.0.0 (2019-09-03)

- No significant changes.

## 1.0.0-rc.1 (2019-08-28)

- Print a Welcome to Pulumi message for users during interactive logins to the Pulumi CLI.
  [#3145](https://github.com/pulumi/pulumi/pull/3145)

- Filter the list of templates shown by default during `pulumi new`.
  [#3147](https://github.com/pulumi/pulumi/pull/3147)

## 1.0.0-beta.4 (2019-08-22)

- Fix a crash when using StackReference from the `1.0.0-beta.3` version of
  `@pulumi/pulumi` and `1.0.0-beta.2` or earlier of the CLI.

- Allow Un/MashalProperties to reject Asset and AssetArchive types. (partial fix
  for https://github.com/pulumi/pulumi-kubernetes/issues/737)

## 1.0.0-beta.3 (2019-08-21)

- When using StackReference to fetch output values from another stack, do not mark a value as secret if it was not
  secret in the stack you referenced. (fixes [#2744](https://github.com/pulumi/pulumi/issues/2744)).

- Allow resource IDs to be changed during `pulumi refresh` operations

- Do not crash when renaming a stack that has never been updated, when using the local backend. (fixes
  [#2654](https://github.com/pulumi/pulumi/issues/2654))

- Fix intermittet "NoSuchKey" issues when using the S3 based backend. (fixes [#2714](https://github.com/pulumi/pulumi/issues/2714)).

- Support filting stacks by organization or tags when using `pulumi stack ls`. (fixes [#2712](https://github.com/pulumi/pulumi/issues/),
  [#2769](https://github.com/pulumi/pulumi/issues/2769)

- Explicitly setting `deleteBeforeReplace` to `false` now overrides the provider's decision.
  [#3118](https://github.com/pulumi/pulumi/pull/3118)

- Fail read steps (e.g. the step generated by a call to `aws.s3.Bucket.get()`) if the requested resource does not exist.
  [#3123](https://github.com/pulumi/pulumi/pull/3123)

## 1.0.0-beta.2 (2019-08-13)

- Fix the package version compatibility checks in the NodeJS language host.
  [#3083](https://github.com/pulumi/pulumi/pull/3083)

## 1.0.0-beta.1 (2019-08-13)

- Do not propagate input properties to missing output properties during preview. The old behavior can cause issues that
  are difficult to diagnose in cases where the actual value of the output property differs from the value of the input
  property, and can cause `apply`s to run at unexpected times. If this change causes issues in a Pulumi program, the
  original behavior can be enabled by setting the `PULUMI_ENABLE_LEGACY_APPLY` environment variable to `true`.

- Fix a bug in the GitHub Actions program preventing errors from being rendered in the Actions log on github.com.
  [#3036](https://github.com/pulumi/pulumi/pull/3036)

- Fix a bug in the Node.JS SDK that caused failure details for provider functions to go unreported.
  [#3048](https://github.com/pulumi/pulumi/pull/3048)

- Fix a bug in the Python SDK that caused crashes when using asynchronous data sources.
  [#3056](https://github.com/pulumi/pulumi/pull/3056)

- Fix crash when exporting secrets from a pulumi app
  [#2962](https://github.com/pulumi/pulumi/issues/2962)

- Fix a panic in logger when a secret contains non-printable characters
  [#3074](https://github.com/pulumi/pulumi/pull/3074)

- Check the uniqueness of the project name during pulumi new
  [#3065](https://github.com/pulumi/pulumi/pull/3065)

## 0.17.28 (2019-08-05)

- Retry renaming a temporary folder during plugin installation
  [#3008](https://github.com/pulumi/pulumi/pull/3008)

- Add support for additional Pulumi secrets providers using AWS KMS, Azure KeyVault, Google Cloud
  KMS and HashiCorp Vault.  These secrets providers can be configured at stack creation time using
  `pulumi stack init b --secrets-provider="awskms://alias/LukeTesting?region=us-west-2"`, and ensure
  that all encrypted data associated with the stack is encrypted using the target cloud platform
  encryption keys.  This augments the previous choice between using the app.pulumi.com-managed
  secrets encryption or a fully-client-side local passphrase encryption.
  [#2994](https://github.com/pulumi/pulumi/pull/2994)

- Add `Output.concat` to Python SDK [#3006](https://github.com/pulumi/pulumi/pull/3006)

- Add `requireOutput` to `StackReference` [#3007](https://github.com/pulumi/pulumi/pull/3007)

- Arbitrary values can now be exported from a Python app. This includes dictionaries, lists, class
  instances, and the like. Values are treated as "plain old python data" and generally kept as
  simple values (like strings, numbers, etc.) or the simple collections supported by the Pulumi data model (specifically, dictionaries and lists).

- Fix `get_secret` in Python SDK always returning None.

- Make `pulumi.runtime.invoke` synchronous in the Python SDK [#3019](https://github.com/pulumi/pulumi/pull/3019)

- Fix a bug in the Python SDK that caused input properties that are coroutines to be awaited twice.
  [#3024](https://github.com/pulumi/pulumi/pull/3024)

### Compatibility

- Deprecated functions in `@pulumi/pulumi` will now issue warnings if you call them.  Please migrate
  off of these functions as they will be removed in a future release.  The deprecated functions are.
  1. `function computeCodePaths(extraIncludePaths?: string[], ...)`.  Use the `computeCodePaths`
     overload that takes a `CodePathOptions` instead.
  2. `function serializeFunctionAsync`. Please use `serializeFunction` instead.

## 0.17.27 (2019-07-29)

- Fix an error message from the logging subsystem which was introduced in v0.17.26
  [#2989](https://github.com/pulumi/pulumi/pull/2997)

- Add support for property paths in `ignoreChanges`, and pass `ignoreChanges` to providers
  [#3005](https://github.com/pulumi/pulumi/pull/3005). This allows differences between the actual and desired
  state of the resource that are not captured by differences in the resource's inputs to be ignored (including
  differences that may occur due to resource provider bugs).

## 0.17.26 (2019-07-26)

- Add `get_object`, `require_object`, `get_secret_object` and `require_secret_object` APIs to Python
  `config` module [#2959](https://github.com/pulumi/pulumi/pull/2959)

- Fix unexpected provider replacements when upgrading from older CLIs and older providers
  [pulumi/pulumi-kubernetes#645](https://github.com/pulumi/pulumi-kubernetes/issues/645)

- Add *Python* support for renaming resources via the `aliases` resource option.  Adding aliases
  allows new resources to match resources from previous deployments which used different names,
  maintaining the identity of the resource and avoiding replacements or re-creation of the resource.
  This was previously added to the *JavaScript* sdk in 0.17.15.
  [#2974](https://github.com/pulumi/pulumi/pull/2974)

## 0.17.25 (2019-07-19)

- Support for Dynamic Providers in Python [#2900](https://github.com/pulumi/pulumi/pull/2900)

## 0.17.24 (2019-07-19)

- Fix a crash when two different versions of `@pulumi/pulumi` are used in the same Pulumi program
  [#2942](https://github.com/pulumi/pulumi/issues/2942)

## 0.17.23 (2019-07-16)
- `pulumi new` allows specifying a local path to templates (resolves
   [#2672](https://github.com/pulumi/pulumi/issues/2672))

- Fix an issue where a file archive created on Windows would contain back-slashes
  [#2784](https://github.com/pulumi/pulumi/issues/2784)

- Fix an issue where output values of a resource would not be present when they
  contained secret values, when using Python.

- Fix an issue where emojis are printed in non-interactive mode. (fixes
  [#2871](https://github.com/pulumi/pulumi/issues/2871))

- Promises/Outputs can now be directly exported as the top-level (i.e. not-named) output of a Stack.
  (fixes [#2910](https://github.com/pulumi/pulumi/issues/2910))

- Add support for importing existing resources to be managed using Pulumi. A resource can be imported
  by setting the `import` property in the resource options bag when instantiating a resource. In order to
  successfully import a resource, its desired configuration (i.e. its inputs) must not differ from its
  actual configuration (i.e. its state) as calculated by the resource's provider.
- Better error message for missing npm on `pulumi new` (fixes [#1511](https://github.com/pulumi/pulumi/issues/1511))

- Add the ability to pass a customTimeouts object from the providers across the engine
  to resource management. (fixes [#2655](https://github.com/pulumi/pulumi/issues/2655))

### Breaking Changes

- Defer to resource providers in all cases where the engine must determine whether or not a resource
  has changed. Note that this can expose bugs in the resources providers that cause diffs to be
  present even if the desired configuration matches the actual state of the resource: in these cases,
  users can set the `PULUMI_ENABLE_LEGACY_DIFF` environment variable to `1` or `true` to enable the
  old diff behavior. https://github.com/pulumi/pulumi/issues/2971 lists the known provider bugs
  exposed by these changes and links to appropriate workarounds or tracking issues.

## 0.17.22 (2019-07-11)

- Improve update performance in cases where a large number of log messages are
  reported during an update.

## 0.17.21 (2019-06-26)

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

## 0.16.3 (2018-11-06)

### Improvements

- Fully support Node 11 [pulumi/pulumi#2101](https://github.com/pulumi/pulumi/pull/2101)

## 0.16.2 (2018-10-29)

### Improvements

- Fix a regression that would cause resource operations to not be processed in parallel when using the latest CLI with a `@pulumi/pulumi` older than 0.16.1 [pulumi/pulumi#2123](https://github.com/pulumi/pulumi/pull/2123)

- Fail with a better error message (and in fewer cases) on Node 11. We hope to have complete support for Node 11 later this week, but for now recommend using Node 10 or earlier. [pulumi/pulumi#2098](https://github.com/pulumi/pulumi/pull/2098)

## 0.16.1 (2018-10-23)

### Improvements

 - A new top-level CLI command “pulumi state” was added to assist in making targeted edits to the state of a stack. Two subcommands, “pulumi state delete” and “pulumi state unprotect”, can be used to delete or unprotect individual resources respectively within a Pulumi stack. [pulumi/pulumi#2024](https://github.com/pulumi/pulumi/pull/2024)
- Default to allowing as many parallel operations as possible [pulumi/pulumi#2065](https://github.com/pulumi/pulumi/pull/2065)
- Fixed an issue with the generated type for an Unwrap expression when using TypeScript [pulumi/pulumi#2061](https://github.com/pulumi/pulumi/pull/2061)
- Improve error messages when resource plugins can't be loaded or when a checkpoint is invalid [pulumi/pulumi#2078](https://github.com/pulumi/pulumi/pull/2078)
- Fix link to the Pulumi Web Console in the CLI for a stack [pulumi/pulumi#2075](https://github.com/pulumi/pulumi/pull/2075)
- Attach git commit metadata to Pulumi updates in some additional cases [pulumi/pulumi#2062](https://github.com/pulumi/pulumi/pull/2062) and [pulumi/pulumi#2069](https://github.com/pulumi/pulumi/pull/2069)

## 0.16.0 (2018-10-15)

### Major Changes

#### Improvements to CLI output

Default colors that fit better for both light and dark terminals.  Overall updates to rendering of previews/updates for consistency and simplicity of the display.

#### Parallelized resource deletion

Parallel resource creation and updates were added in `0.15`.  In `0.16`, this has been extended to include deletions, which are now conservatively parallelized based on dependency information. [pulumi/pulumi#1963](https://github.com/pulumi/pulumi/pull/1963)

#### Support for any CI system in the Pulumi GitHub App

The Pulumi GitHub App previously supported just TravisCI.  With this release the `pulumi` CLI now supports configurable CI providers via environment variables.  Thanks [@jen20](https://github.com/jen20)!

### Improvements

In addition to the above features, we've made a handfull of day to day improvements in the CLI:
- Support for `zsh` completions. Thanks to [@Tirke](https://github.com/Tirke)!) [pulumi/pulumi#1967](https://github.com/pulumi/pulumi/pull/1967)
- JSON formatting support for `pulumi stack output`. [pulumi/pulumi#2000](https://github.com/pulumi/pulumi/pull/2000)
- Added a `Dockerfile` for the Pulumi CLI and development environment for use in hosted environments.
- Many improvements for Go development.  Thanks to [@justone](https://github.com/justone)!. [pulumi/pulumi#1954](https://github.com/pulumi/pulumi/pull/1954) [pulumi/pulumi#1955](https://github.com/pulumi/pulumi/pull/1955) [pulumi/pulumi#1965](https://github.com/pulumi/pulumi/pull/1965)
- Extend `pulumi.output` to deeply unwrap `Input`s.  This significantly simplifies working with `Inputs` when building Pulumi components.  [pulumi/pulumi#1915](https://github.com/pulumi/pulumi/pull/1915)

## 0.15.4 (2018-09-28)

### Improvements

- Fix an assert in display code when a resource property transitions from an asset to an archive or the other way around

## 0.15.3 (2018-09-18)

### Improvements

- Improved performance of `pulumi stack ls`
- Fix build authoring so the dynamic provider works for the CLI built by Homebrew (thanks to **[@Tirke](https://github.com/Tirke)**!)

## 0.15.2 (2018-09-11)

### Major Changes

Major features of this release include:

#### Ephemeral status messages

Providers are now able to register "ephmeral" update messages which are shown in the "Info" column in the CLI during an update, but which are not printed at the end of the update. The new version of the `@pulumi/kubernetes` package uses this when printing messages about resource initialization.

#### Local backend

The local backend (which stores your deployment's state file locally, instead of on pulumi.com) has been improved. You can now use `pulumi login --local` or `pulumi login file://<path-to-storage-root>` to select the local backend and control where state files are stored. In addition, older versions of the CLI would behave slightly differently when using the local backend vs pulumi.com, for example, some operations would not show previews before running.  This has been fixed.  When using the local backend, updates print the on disk location of the checkpoint file that was written. The local backend is covered in more detail in [here](https://www.pulumi.com/docs/reference/state/).

#### `pulumi refresh`

We've made a bunch of improvements in `pulumi refresh`. Some of these improve the UI during a refresh (for example, clarifying text about the underyling operations) as well fixing bugs with refreshing certain types of objects (for example CloudFront CDNs).

#### `pulumi up` and `pulumi new`

You can now pass a URL to a Git repository to `pulumi up <url>` to deploy a project without having to manage its source code locally. This works like `pulumi new <url>`, but configures and deploys the project from a temporary directory that will be cleaned up automatically after the update.

`pulumi new` now outputs an error when the current working directory (or directory specified explicitly via the `--dir` flag) is not empty. Additionally, `pulumi new` now runs a preview of an initial update at the end of its operation and asks if you would like to perform the update.

Both `pulumi up` and `pulumi new` now support `-c` flags for specifying config values as arguments (e.g. `pulumi up <url> -c aws:region=us-east-1`).

### Improvements

In addition to the above features, we've made a handfull of day to day improvements in the CLI:

- Support `pulumi` in a projects in a Yarn workspaces. [pulumi/pulumi#1893](https://github.com/pulumi/pulumi/pull/1893)
- Improve error message when there are errors decrypting secret configuration values. [pulumi/pulumi#1815](https://github.com/pulumi/pulumi/pull/1815)
- Don't fail `pulumi up` when plugin discovery fails. [pulumi/pulumi#1745](https://github.com/pulumi/pulumi/pull/1745)
- New helpers for extracting and validating values from `pulumi.Config`. [pulumi/pulumi#1843](https://github.com/pulumi/pulumi/pull/1843)
- Support serializng "factory" functions. [pulumi/pulumi#1804](https://github.com/pulumi/pulumi/pull/1804)

## 0.15.0 (2018-08-13)

### Major Changes

#### Parallelism

Pulumi now performs resource creates and updates in parallel, driven by dependencies in the resource graph. (Parallel deletes are coming in a future release.) If your program has implicit dependencies that Pulumi does not already see as dependencies, it's possible parallel will cause ordering issues. If this happens, you may set the `dependsOn` on property in the `resourceOptions` parameter to any resource. By default, Pulumi allows 10 parallel operations, but the `-p` flag can be used to override this. `-p=1` disables parallelism altogether. Parallelism is supported for Node.js and Go programs, and Python support will come in a future release.

#### First Class Providers

Pulumi now allows creation and configuration of resource providers programmatically. In addition to the default provider instance for each resource, you can also create an explicit version of the provider and configure it explicitly. This can be used to create some resources in a different region from your main deployment, or deploy resources to a programmatically configured Kubernetes cluster, for example. We have [a multi-region deployment example](https://github.com/pulumi/pulumi-aws/blob/master/examples/multiple-regions/index.ts) for illustrative purposes.

#### Status Rich Updates

The Pulumi CLI is now able to report more detailed information from individual resources during an update. This is used, for instance, in the Kubernetes provider, to provide incremental progress output for steps that may take a while to comeplete (such as deployment orchestration). We anticipate leveraging this feature in more places over time.

#### Improved Templating Support

You can now pass a URL to a Git repository to `pulumi new` to install a custom template, enabling you to share common templates across your team. If you pass a simple name, or omit arguments altogether, `pulumi new` behaves as before, using the [templates hosted by Pulumi](https://github.com/pulumi/templates).

#### Native TypeScript support

By default, Pulumi now natively supports TypeScript, so you do not need to run `tsc` explicitly before deploying. (We often forget to do this too!) Simply run `pulumi up`, and the program will be recompiled on the fly before running it.

To use this new support, upgrade your `@pulumi/pulumi` version to 0.15.0, in addition to the CLI. Pulumi prefers JavaScript source to TypeScript source, so if you had been using TypeScript previously, we recommend you make the following changes:

1. Remove the `main` and `typings` directives from `package.json`, as well as the `build` script.
2. Remove the `bin` folder that contained your previously compiled code.
3. You may remove the dependency on `typescript` from your `package.json` as well, since `@pulumi/pulumi` has one.

While a `tsconfig.json` file is no longer required, as Pulumi uses intelligent defaults, other tools like VS Code behave
better when it is present, so you'll probably want to keep it.

#### Closure capturing improvements

We've improved our closure capturing logic, which should allow you to write more idiomatic code in lambda functions that are uploaded to the cloud. Previously, if you wanted to use a module, we required you to write either `require('module')` or `await import('module')` inside your lambda function. In addition, if you wanted to use a helper you defined in another file, you had to require that module in your function as well. With these changes, the following code now works:

```typescript
import * as axios from "axios";
import * as cloud from "@pulumi/cloud-aws";

const api = new cloud.API("api");
api.get("/", async (req, res) => {
    const statusText = (await axios.default.get("https://www.pulumi.com")).statusText;
    res.write(`GET https://www.pulumi.com/ == ${statusText}`).end();
});
```

#### Default value for configuration package

The `pulumi.Config` object can now be created without an argument. When no argument is supplied, the value of the current project is used. This means that application level code can simply do `new pulumi.Confg()` without passing any argument. For library authors, you should continue to pass the name of your package as an argument.

#### Pulumi GitHub App (preview)

The Pulumi GitHub application bridges the gap between GitHub (source code, pull requests) and Pulumi (cloud resources, stack updates). By installing
the Pulumi GitHub application into your GitHub organization, and then running Pulumi as part of your CI build process, you can now see the results of
stack updates and previews as part of pull requests. This allows you to see the potential impact a change would have on your cloud infrastructure before
merging the code.

The Pulumi GitHub application is still in preview as we work to support more CI systems and provide richer output. For information on how to install the
GitHub application and configure it with your CI system, please [visit our documentation](https://www.pulumi.com/docs/reference/cd-github/) page.

### Improvements

- The CLI no longer emits warnings if it can't detect metadata about your git enlistement (for example, what GitHub project it coresponds to).
- The CLI now only warns about adding a plaintext configuration in cases where it appears likely you may be storing a secret.

## 0.14.3 (2018-07-20)

### Improvements

#### Fixed

- Support empty text assets ([pulumi/pulumi#1599](https://github.com/pulumi/pulumi/pull/1599)).

- When printing message in non-interactive mode, do not keep printing out the worst diagnostic ([pulumi/pulumi#1640](https://github.com/pulumi/pulumi/pull/1640)). When run in non interactive environments (e.g. docker) Pulumi would print duplicate messages to the screen related to a resource when the running Pulumi program was writing to standard out (e.g. if it was invoking a docker build). This no longer happens. The full output from the program continues to be printed at the end of execution.

- Work around a potentially bad assert in the engine ([pulumi/pulumi#1640](https://github.com/pulumi/pulumi/pull/1644)). In some cases, when Pulumi failed to delete a resource as part of an update, future updates would crash with an assert message. This is no longer the case and Pulumi will try to delete the resource it had marked as should be deleted.

#### Added

- Print out a 'still working' message every 20 seconds when in non-interactive mode ([pulumi/pulumi#1616](https://github.com/pulumi/pulumi/pull/1616)). When Pulumi is waiting for a long running resource operation to create (e.g. waiting for an ECS service to become stable after creation), print some output to the console even when running non-interactively. This helps for cases like TravsCI where if output is not written for a while the job is assumed to have hung and is aborted.

- Support the NO_COLOR env variable to suppress any colored output ([pulumi/pulumi#1594](https://github.com/pulumi/pulumi/pull/1594)). Pulumi now respects the `NO_COLOR` environment variable. When set to a truthy value, colors are suppressed from the CLI. In addition, the `--color` flag can now be passed to all `pulumi` commands.

## 0.14.2 (2018-07-03)

### Improvements

- Support -s in `stack {export, graph, import, output}` ([pulumi/pulumi#1572](https://github.com/pulumi/pulumi/pull/1574)). `pulumi stack export`, `pulumi stack graph`, `pulumi stack import` and `pulumi stack output` now support a `-s` or `--stack` flag, which allows them to operate on a different stack that the currently selected one.

## 0.14.1 (2018-06-29)

### Improvements

#### Added

- Add `pulumi whoami` ([pulumi/pulumi#1572](https://github.com/pulumi/pulumi/pull/1572)). `pulumi whoami` will report the account name of the current logged in user. In addition, we now display the name of the current user after `pulumi login`.

#### Fixed

- Don't require `PULUMI_DEBUG_COMMANDS` to be set to use local backend ([pulumi/pulumi#1575](https://github.com/pulumi/pulumi/pull/1575)).

- Improve misleading `pulumi new` summary message ([pulumi/pulumi#1571](https://github.com/pulumi/pulumi/pull/1571)).

- Fix printing out outputs in a pulumi program ([pulumi/pulumi#1531](https://github.com/pulumi/pulumi/pull/1531)). Pulumi now shows the values of output properties after a `pulumi up` instead of requiring you to run `pulumi stack output`.

- Do a better job preventing serialization of unnecessary objects in closure serialization ([pulumi/pulumi#1543](https://github.com/pulumi/pulumi/pull/1543)).  We've improved our analysis when serializing functions. This yeilds smaller code when a function is serialized and prevents errors around unused native code being captured in some cases.

## 0.14.0 (2018-06-15)

### Improvements

#### Added

- Publish to pypi.org ([pulumi/pulumi#1497](https://github.com/pulumi/pulumi/pull/1497)). Pulumi packages are now public on pypi.org!

- Add optional `--dir` flag to `pulumi new` ([pulumi/pulumi#1459](https://github.com/pulumi/pulumi/pull/1459)). The `pulumi new` command now has an optional flag `--dir`, for the directory to place the generated project. If it doesn't exist, it will be created.

- Support Pulumi programs written in Go ([pulumi/pulumi#1456](https://github.com/pulumi/pulumi/pull/1456)). Initial version for Pulumi programs written in Go. While it is not complete, basic resource registration works.

- Allow overriding config location ([pulumi/pulumi#1379](https://github.com/pulumi/pulumi/pull/1379)). Support a new `config` member in `Pulumi.yaml`, which specifies a relative path to a folder where per-stack configuration is stored. The path is relative to the location of `Pulumi.yaml` itself.

- Delete existing resources before replacing, for resources that must be singletons ([pulumi/pulumi#1365](https://github.com/pulumi/pulumi/pull/1365)). For resources where the cloud vendor does not allow multiple resources to exist, such as a mount target in EFS, Pulumi now deletes the existing resource before creating a replacement resource.

#### Changed

- Compute required packages during closure serialization ([pulumi/pulumi#1457](https://github.com/pulumi/pulumi/pull/1457)). Closure serialization now keeps track of the `require`'d packages it sees in the function bodies that are serialized during a call to `serializeFunction`. So, only required packages are uploaded to Lambda.

- Support browser based logins to the CLI ([pulumi/pulumi#1439](https://github.com/pulumi/pulumi/pull/1439)). The Pulumi CLI now has an option to login via a browser. When you are prompted for an access token, you can just hit enter. The CLI then opens a browser to [app.pulumi.com](https://app.pulumi.com) so that you can authenticate.

#### Fixed

- Support better previews in Python by mocking out Unknown values ([pulumi/pulumi#1482](https://github.com/pulumi/pulumi/pull/1482)). During the preview phase of a deployment, computed values were `Unknown` in Python, causing the preview to be empty. This issue is now resolved.

- Issue a better error message if you capture a V8 intrinsic ([pulumi/pulumi#1423](https://github.com/pulumi/pulumi/pull/1423)). It's possible to accidentally take a dependency on a Pulumi deployment-time library, which causes problems when creating a runtime function for AWS Lambda. There is now a better error message when this situation occurs.

## 0.12.2 (2018-05-19)

### Improvements

- Improve the promise leak experience ([pulumi/pulumi#1374](https://github.com/pulumi/pulumi/pull/1374)). Fixes an issue where a promise leak could be erroneously reported. Also, show simple error message by default, unless the environment variable `PULUMI_DEBUG_PROMISE_LEAKS` is set.

## 0.12.1 (2018-05-09)

### Added

- A new all-in-one installer script is now available at [https://get.pulumi.com](https://get.pulumi.com).

- Many enhancements to `pulumi new` ([pulumi/pulumi#1307](https://github.com/pulumi/pulumi/pull/1307)).  The command now interactively walks through creating everything needed to deploy a new stack, including selecting a template, providing a name, creating a stack, setting default configuration, and installing dependencies.

- Several improvements to the `pulumi up` CLI experience ([pulumi/pulumi#1260](https://github.com/pulumi/pulumi/pull/1260)): a tree view display, more details from logs during deployments, and rendering of stack outputs at the end of updates.

### Changed

- (**Breaking**) Remove the `--preview` flag in `pulumi up`, in favor of reintroducing `pulumi preview` ([pulumi/pulumi#1290](https://github.com/pulumi/pulumi/pull/1290)). Also, to accept an update without the interactive prompt, use the `--yes` flag, rather than `--force`.

### Fixed

- Significant performance improvements for `pulumi up` ([pulumi/pulumi#1319](https://github.com/pulumi/pulumi/pull/1319)).

- JavaScript `async` functions in Node 7.6+ now work with Pulumi function serialization ([pulumi/pulumi#1311](https://github.com/pulumi/pulumi/pull/1311).

- Support installation on Windows in folders which contain spaces in their name ([pulumi/pulumi#1300](https://github.com/pulumi/pulumi/pull/1300)).


## 0.12.0 (2018-04-26)

### Added

- Add a `pulumi cancel` command ([pulumi/pulumi#1230](https://github.com/pulumi/pulumi/pull/1230)). This command cancels any in-progress operation for the current stack.

### Changed

- (**Breaking**) Eliminate `pulumi init` requirement ([pulumi/pulumi#1226](https://github.com/pulumi/pulumi/pull/1226)). The `pulumi init` command is no longer required and should not be used for new stacks. For stacks created prior to the v0.12.0 SDK, `pulumi init` should still be run in the project directory if you are connecting to an existing stack. For new projects, stacks will be created under the currently logged in account. After upgrading the CLI, it is necessary to run `pulumi stack select`, as the location of bookkeeping files has been changed. For more information, see [Creating Stacks](https://www.pulumi.com/docs/reference/stack/#create-stack).

- (**Breaking**) Remove the explicit 'pulumi preview' command ([pulumi/pulumi#1170](https://github.com/pulumi/pulumi/pull/1170)). The `pulumi preview` output has now been merged in to the `pulumi up` command. Before an update is run, the preview is shown and you can choose whether to proceed or see more update details. To see just the preview operation, run `pulumi up --preview`.

- Switch to a more streamlined view for property diffs in `pulumi up` ([pulumi/pulumi#1212](https://github.com/pulumi/pulumi/pull/1212)).

- Allow multiple versions of the `@pulumi/pulumi` package to be loaded ([pulumi/pulumi#1209](https://github.com/pulumi/pulumi/pull/1209)). This allows packages and dependencies to be versioned independently.

### Fixed
- When running a `pulumi up` or `destroy` operation, a single ctrl-c will cancel the current operation, waiting for it to complete. A second ctrl-c will terminate the operation immediately. ([pulumi/pulumi#1231](https://github.com/pulumi/pulumi/pull/1231)).

- When getting update logs, get all results ([pulumi/pulumi#1220](https://github.com/pulumi/pulumi/pull/1220)). Fixes a bug where logs could sometimes be truncated in the pulumi.com console.

## 0.11.3 (2018-04-13)

- Switch to a resource-progress oriented view for pulumi preview, update, and destroy operations ([pulumi/pulumi#1116](https://github.com/pulumi/pulumi/pull/1116)). The operations `pulumi preview`, `update` and `destroy` have far simpler output by default, and show a progress view of ongoing operations. In addition, there is a structured component view, showing a parent operation as complete only when all child resources have been created.

- Remove strict dependency on Node v6.10.x ([pulumi/pulumi#1139](https://github.com/pulumi/pulumi/pull/1139)). It is now no longer necessary to use a specific version of Node to run Pulumi programs. Node versions after 6.10.x are supported, as long as they are under **Active LTS** or are the **Current** stable release.

## 0.11.2 (2018-04-06)

### Changed

- (Breaking) Require `pulumi login` before commands that need a backend ([pulumi/pulumi#1114](https://github.com/pulumi/pulumi/pull/1114)). The `pulumi` CLI now requires you to log in to pulumi.com for most operations.

### Fixed

- Improve the error message arising from missing required configurations for resource providers ([pulumi/pulumi#1097](https://github.com/pulumi/pulumi/pull/1097)). The error message now prints all missing configuration keys, along with their descriptions.

## 0.11.0 (2018-03-20)

### Added

- Add a `pulumi new` command to scaffold a project ([pulumi/pulumi#1008](https://github.com/pulumi/pulumi/pull/1008)). Usage is `pulumi new [templateName]`. If template name is not specified, the CLI will prompt with a list of templates. Currently, the templates `javascript`, `python` and `typescript` are available. Templates are defined in the GitHub repo [pulumi/templates](https://github.com/pulumi/templates) and contributions are welcome!

- Python is now a supported language in Pulumi ([pulumi/pulumi#800](https://github.com/pulumi/pulumi/pull/800)). For more information, see [Python documentation](https://www.pulumi.com/docs/reference/python/).

### Changed

- (Breaking) Change the way that configuration is stored ([pulumi/pulumi#986](https://github.com/pulumi/pulumi/pull/986)). To simplify the configuration model, there is no longer a separate notion of project and workspace settings, but only stack settings. The switches `--all` and `--save` are no longer supported; any common settings across stacks must be set on each stack directly. Settings for a stack are stored in a file that is a sibling to `Pulumi.yaml`, named `Pulumi.<stack-name>.yaml`. On first run `pulumi`, will migrate projects from the previous configuration format to the new one. The recommended practice is that developer stacks that are not shared between team members should be added to `.gitignore`, while stack setting files for shared stacks should be checked in to source control. For more information, see the section [Defining and setting stack settings](https://www.pulumi.com/docs/reference/config/#config-stack).

- (Breaking) Eliminate the superfluous `:config` part of configuration keys ([pulumi/pulumi#995](https://github.com/pulumi/pulumi/pull/995)). `pulumi` no longer requires configuration keys to have the string `:config` in them. Using the `:config` string in keys for the object `@pulumi/pulumi.Config` is deprecated and `preview` and `update` show warnings when it is used. Additionally, it is preferred to set keys in the form `aws:region` rather than `aws:config:region`. For compatibility, the old behavior is also supported, but will be removed in a future release. For more information, see the article [Configuration](https://www.pulumi.com/docs/reference/config/).

- (Breaking) Modules are treated as normal values when serialized ([pulumi/pulumi#1030](https://github.com/pulumi/pulumi/pull/1030)). If you need to use a module at runtime, consider either using `require` or `await import` at runtime, or pre-compute what you need and capture the resulting data or objects.

<!-- NOTE: the programming-model article below is still all todos. -->
- (Breaking) Serialize resource registration after inputs resolve ([pulumi/pulumi#964](https://github.com/pulumi/pulumi/pull/964)). Previously, resources were most often created/updated in the order they were seen during the Pulumi program execution. In preparation for supporting parallel resource operations, these operations now run in an order that respects the dependencies between resources (via [`Output`](https://www.pulumi.com/docs/reference/pkg/nodejs/pulumi/pulumi/#Output)), but may not match the order of program execution. This is mostly transparent to Pulumi program authors, but does mean that any missing dependencies will cause your program to fail in unexpected ways. For more information on how such failures manifest and what to do about them, see the article [Programming Model](https://www.pulumi.com/docs/reference/programming-model/).

- Hide secrets from CLI output ([pulumi/pulumi#1002](https://github.com/pulumi/pulumi/pull/1002)). To prevent secret values from being accidentally disclosed in command output or logs, `pulumi` replaces secret values with the string `[secret]`. Inspired by the behavior of [Travis CI](https://travis-ci.org/).

- Change default of where stacks are created ([pulumi/pulumi#971](https://github.com/pulumi/pulumi/pull/971)). If currently logged in to the Pulumi CLI, `stack init` creates a managed stack; otherwise, it creates a local stack. To force a local or remote stack, use the flags `--local` or `--remote`.

### Fixed

- Improve error messages output by the CLI ([pulumi/pulumi#1011](https://github.com/pulumi/pulumi/pull/1011)). RPC endpoint errors have been improved. Errors such as "catastrophic error" and "fatal error" are no longer duplicated in the output.

- Produce better error messages when the main module is not found ([pulumi/pulumi#976](https://github.com/pulumi/pulumi/pull/976)). If you're running TypeScript but have not run `tsc` or your main JavaScript file does not exist, the CLI will print a helpful `info:` message that points to the possible source of the error.

## 0.10.0 (2018-02-27)

> **Note:** The v0.10.0 SDK has a strict dependency on Node.js 6.10.2.

### Added

- Support "force" option when deleting a managed stack.

- Add a `pulumi history` command ([pulumi#636](https://github.com/pulumi/pulumi/issues/636)). For a managed stack, use the `pulumi history` to view deployments of that stack's resources.

### Changed

- (Breaking) Use `npm install` instead of `npm link` to reference the Pulumi SDK `@pulumi/aws`, `@pulumi/cloud`, `@pulumi/cloud-aws`. For more information, see [Pulumi npm packages](https://www.pulumi.com/docs/reference/pkg/nodejs/).

- (Breaking) Explicitly track resource dependencies via `Input` and `Output` types. This enables future improvements to the Pulumi development experience, such as parallel resource creation and enhanced dependency visualization. When a resource is created, all of its output properties are instances of a new type [`pulumi.Output<T>`](https://www.pulumi.com/docs/reference/pkg/nodejs/pulumi/pulumi/#Output). `Output<T>` contains both the value of the resource property and metadata that tracks resource dependencies. Inputs to a resource now accept `Output<T>` in addition to `T` and `Promise<T>`.

### Fixed

- Managed stacks sometimes return a 500 error when requesting logs
- Error when using `float64` attributes using SDK v0.9.9 ([pulumi-terraform#95](https://github.com/pulumi/pulumi-terraform/issues/95))
- `pulumi logs` entries only return first line ([pulumi#857](https://github.com/pulumi/pulumi/issues/857))

## 0.9.13 (2018-02-07)

### Added

- Added the ability to control the upload context to the Pulumi Service. You may now set a `context` property in `Pulumi.yaml`, which is combined with the location of `Pulumi.yaml`. This new path is the root of what is uploaded and can be used during deployment. This allows you to, for example, share common code that is located in a folder in your source tree above the directory `Pulumi.yaml` for the project you are deploying.

- Added additional configuration for docker builds for a container. The `build` property of a container may now either be a string (which is treated as a path to the folder to do a `docker build` in) or an object with properties `context`, `dockerfile` and `args`, which are passed to `docker build`. If unset, `context` defaults to the current working directory, `dockerfile` defaults to `Dockerfile` and `args` default to no arguments.

## 0.9.11 (2018-01-22)

### Added

- Added the ability to import or export a stack's deployment in the Pulumi CLI. This command can be used for either local or managed stacks. There are two new verbs under the command `stack`:
   - `export` writes the current stack's latest deployment to stdout in JSON format.
   - `import` reads a new JSON deployment from stdin and applies it to the current stack.

- A basic progress spinner is displayed during deployment operations.
   - When the Pulumi CLI is run in interactive mode, it displays an animated ASCII spinner
   - When run in non-interactive mode, CLI prints a message that it is still working. For CI systems that kill jobs when there is no CLI output (such as TravisCI), this eliminates the need to create shell scripts that periodically print output.

### Changed

- To make the behavior of local and managed stacks consistent, the Pulumi CLI uses a separate encryption key for each stack, rather than one shared for all stacks. You can now use a different passphrase for different stacks. Similar to managed stacks, you cannot copy and paste an encrypted value from one stack to another in `Pulumi.yaml`. Instead you must manage the value via `pulumi config`.

- The default behavior for `--color` is now `always`. To change this, specify `--color always` or `--color never`. Previously, the value was based on the presence of the flag `--debug`.

- The command `pulumi logs` now defaults to returning one hour of logs and outputs the start time that is  used.

### Fixed

- When a stack is removed, `pulumi` now deletes any configuration it had saved in either the `Pulumi.yaml` file or the workspace.

## 0.9.8 (2017-12-28)

### Added

#### Pulumi Console and managed stacks

New in this release is the [Pulumi Console](https://app.pulumi.com) and stacks that are managed by Pulumi. This is the recommended way to safely deploy cloud applications.
- `pulumi stack init` now creates a Pulumi managed stack. For a local stack, use `--local`.
- All Pulumi CLI commands now work with managed stacks. Login to Pulumi via `pulumi login`.
- The [Pulumi Console](https://app.pulumi.com) provides a management experience for stacks. You can view the currently deployed resources (along with the AWS ARNs) and see logs from the last update operation.

#### Components and output properties

- Support for component resources([pulumi #340](https://github.com/pulumi/pulumi/issues/340)), enabling grouping of resources into logical components. This provides an improved view of resources during `preview` and `update` operations in the CLI ([pulumi #417](https://github.com/pulumi/pulumi/issues/417)).

  ```
  + pulumi:pulumi:Stack: (create)
     [urn=urn:pulumi:donna-testing::url-shortener::pulumi:pulumi:Stack::url-shortener-donna-testing]
     + cloud:table:Table: (create)
        [urn=urn:pulumi:donna-testing::url-shortener::cloud:table:Table::urls]
        + aws:dynamodb/table:Table: (create)
              [urn=urn:pulumi:donna-testing::url-shortener::cloud:table:Table$aws:dynamodb/table:Table::urls]
  ```
- A stack can have *output properties*, defined as `export let varName = val`. You can view the last deployed value for the output property using `pulumi stack output varName` or in the Pulumi Console.

#### Resource naming

Resource naming is now more consistent, but there is a new file format for checkpoint files for both local and managed stacks.

> If you created stacks in the 0.8 release, you should destroy them with the 0.8 CLI, then recreate with the 0.9.x CLI.

#### Support for configuration secrets
- Store secrets securely in configuration via `pulumi config set --secret`. chris
- The verbs for `config` are now consistent, via `get`, `set`, and `rm`. See [Consistent config verbs #552](https://github.com/pulumi/pulumi/issues/552).

#### Logging

- [**experimental**] Support for the `pulumi logs` command ([pulumi #527](https://github.com/pulumi/pulumi/issues/527)). These features now work:
   - To see new logs as they arrive, use `--follow`
   - Use `--since` to limit to recent logs, such as `pulumi logs --since=1h`
   - Filter to specific resources with `--resource`. This filters to a particular component and its child resources (if any), such as `pulumi logs --resource examples-todoc57917fa --since 1h`

#### Other features

- Support for `.pulumiignore`, for files that should not be uploaded when deploying a managed stack through Pulumi.
- [Allow overriding a `Pulumi.yaml`'s entry point #575](https://github.com/pulumi/pulumi/issues/575). To specify the entry directory, specify `main` in `Pulumi.yaml`. For instance, `main: a/path/to/main/`.
- Support for *protected* resources. A resource can be marked as `protect: true`, which prevents deletion of the resource. For example, `let res = new MyResource("precious", { .. }, { protect: true });`. To "unprotect" the resource, change `protect: false` then run `pulumi up`. See [Allow resources to be flagged "protected" #689](https://github.com/pulumi/pulumi/issues/689).
- Changed defaults for workspace and stack configuration. See [Workspace configuration is error prone #714](https://github.com/pulumi/pulumi/issues/714).
- [Save configuration under the stack by default](https://github.com/pulumi/pulumi/issues/693).

### Fixed
- Improved SDK installer. It automatically creates directories as needed, configures node modules, and prints out friendly error messages.
- [Better diffing in CLI output, especially for Lambdas \#454](https://github.com/pulumi/pulumi/issues/454)
- [`main` does not set working dir correctly for Lambda zip #667](https://github.com/pulumi/pulumi/issues/667)
- [Better error when invalid access token is used in `pulumi login` #640](https://github.com/pulumi/pulumi/issues/640)
- [Eliminate the top-level Stack from all URNs #647](https://github.com/pulumi/pulumi/issues/647)
- Service API for Encrypting and Decrypting secrets
- [Make CLI resilient to network flakiness #763](https://github.com/pulumi/pulumi/issues/763)
- Support --since and --resource on `pulumi logs` when targeting the service
- [Pulumi unable to serialize non-integers #694](https://github.com/pulumi/pulumi/issues/694)
- [Stop buffering CLI output #660](https://github.com/pulumi/pulumi/issues/660)
