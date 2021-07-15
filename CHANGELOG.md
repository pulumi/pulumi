CHANGELOG
=========

## 3.7.0 (2021-07-13)

### Improvements

- [sdk/nodejs] - Support for calling resource methods
  [#7377](https://github.com/pulumi/pulumi/pull/7377)

- [sdk/go] - Support for calling resource methods
  [#7437](https://github.com/pulumi/pulumi/pull/7437)

### Bug Fixes

- [codegen/go] - Reimplement strict go enums to be Inputs.
  [#7383](https://github.com/pulumi/pulumi/pull/7383)

- [codegen/go] - Emit To[ElementType]Output methods for go enum output types
  [#7499](https://github.com/pulumi/pulumi/pull/7499)

- [sdk/nodejs] - Wait on remote component dependencies
  [#7541](https://github.com/pulumi/pulumi/pull/7541)

## 3.6.1 (2021-07-07)

### Improvements

- [sdk] - Add `replaceOnChanges` resource option.
  [#7226](https://github.com/pulumi/pulumi/pull/7226)

- [sdk/go] - Support for authoring resource methods in Go
  [#7379](https://github.com/pulumi/pulumi/pull/7379)

### Bug Fixes

- [sdk/python] - Fix an issue where dependency keys were incorrectly translates to camelcase
  [#7443](https://github.com/pulumi/pulumi/pull/7443)

- [cli] - Fix rendering of diffs for resource without DetailedDiffs
  [#7500](https://github.com/pulumi/pulumi/pull/7500)

## 3.6.0 (2021-06-30)

### Improvements

- [cli] - Added support for passing custom paths that need
  to be watched by the `pulumi watch` command.
  [#7115](https://github.com/pulumi/pulumi/pull/7247)

- [auto/nodejs] - Fail early when multiple versions of `@pulumi/pulumi` are detected in nodejs inline programs.'
  [#7349](https://github.com/pulumi/pulumi/pull/7349)

- [sdk/go] - Add preliminary support for unmarshaling plain arrays and maps of output values.
  [#7369](https://github.com/pulumi/pulumi/pull/7369)

- Initial support for resource methods (Node.js authoring, Python calling)
  [#7363](https://github.com/pulumi/pulumi/pull/7363)

### Bug Fixes

- [sdk/dotnet] - Fix swallowed nested exceptions with inline program, so they correctly bubble to the consumer.
  [#7323](https://github.com/pulumi/pulumi/pull/7323)

- [sdk/go] - Specify known when creating outputs for `construct`.
  [#7343](https://github.com/pulumi/pulumi/pull/7343)

- [cli] - Fix passphrase rotation.
  [#7347](https://github.com/pulumi/pulumi/pull/7347)

- [multilang/python] - Fix nested module generation.
  [#7353](https://github.com/pulumi/pulumi/pull/7353)

- [multilang/nodejs] - Fix a hang when an error is thrown within an apply in a remote component.
  [#7365](https://github.com/pulumi/pulumi/pull/7365)

- [codegen/python] - Include enum docstrings for python.
  [#7374](https://github.com/pulumi/pulumi/pull/7374)


## 3.5.1 (2021-06-16)

**Please Note:** Release v3.5.0 failed in our build pipeline so will be rebuilt with a new tag of v3.5.1

### Improvements

- [dotnet/sdk] Support microsoft logging extensions with inline programs
  [#7117](https://github.com/pulumi/pulumi/pull/7117)

- [dotnet/sdk] Add create unknown to output utilities.
  [#7173](https://github.com/pulumi/pulumi/pull/7173)

- [dotnet] Fix Resharper code issues.
  [#7178](https://github.com/pulumi/pulumi/pull/7178)

- [codegen] - Include properties with an underlying type of string on Go provider instances.

- [cli] - Provide a more helpful error instead of panicking when codegen fails during import.
  [#7265](https://github.com/pulumi/pulumi/pull/7265)

- [codegen/python] - Cache package version for improved performance.
  [#7293](https://github.com/pulumi/pulumi/pull/7293)

- [sdk/python] - Reduce `log.debug` calls for improved performance
  [#7295](https://github.com/pulumi/pulumi/pull/7295)

### Bug Fixes

- [sdk/dotnet] - Fix resources destroyed after exception thrown during inline program
  [#7299](https://github.com/pulumi/pulumi/pull/7299)

- [sdk/python] - Fix regression in behaviour for `Output.from_input({})`

- [sdk/python] - Prevent infinite loops when iterating `Output` objects
  [#7288](https://github.com/pulumi/pulumi/pull/7288)

- [codegen/python] - Rename conflicting ResourceArgs classes
  [#7171](https://github.com/pulumi/pulumi/pull/7171)

## 3.4.0 (2021-06-05)

### Improvements

- [dotnet/sdk] Add get value async to output utilities.
  [#7170](https://github.com/pulumi/pulumi/pull/7170)

### Bug Fixes

- [CLI] Fix broken venv for Python projects started from templates
  [#6624](https://github.com/pulumi/pulumi/pull/6623)

- [cli] - Send plugin install output to stderr, so that it doesn't
  clutter up --json, automation API scenarios, and so on.
  [#7115](https://github.com/pulumi/pulumi/pull/7115)

- [cli] Protect against panics when using the wrong resource type with `pulumi import`
  [#7202](https://github.com/pulumi/pulumi/pull/7202)

- [auto/nodejs] - Emit warning instead of breaking on parsing JSON events for automation API.
  [#7162](https://github.com/pulumi/pulumi/pull/7162)

- [sdk/python] Improve performance of `Output.from_input` and `Output.all` on nested objects.
  [#7175](https://github.com/pulumi/pulumi/pull/7175)

### Misc
- Update version of go-cloud used by Pulumi to `0.23.0`.
  [#7204](https://github.com/pulumi/pulumi/pull/7204)


## 3.3.1 (2021-05-25)

### Improvements

- [dotnet/sdk] - Use source context with serilog
  [#7095](https://github.com/pulumi/pulumi/pull/7095)

- [auto/dotnet] - Make StackDeployment.FromJsonString public
  [#7067](https://github.com/pulumi/pulumi/pull/7067)

- [sdk/python] - Generated SDKs may now be installed from in-tree source.
  [#7097](https://github.com/pulumi/pulumi/pull/7097)

### Bug Fixes

- [auto/nodejs] - Fix an intermittent bug in parsing JSON events
  [#7032](https://github.com/pulumi/pulumi/pull/7032)

- [auto/dotnet] - Fix deserialization of CancelEvent in .NET 5
  [#7051](https://github.com/pulumi/pulumi/pull/7051)

- Temporarily disable warning when a secret config is read as a non-secret
  [#7129](https://github.com/pulumi/pulumi/pull/7129)

## 3.3.0 (2021-05-20)

### Improvements

- [cli] - Provide user information when protected resources are not able to be deleted
  [#7055](https://github.com/pulumi/pulumi/pull/7055)

- [cli] - Error instead of panic on invalid state file import
  [#7065](https://github.com/pulumi/pulumi/pull/7065)

- Warn when a secret config is read as a non-secret
  [#6896](https://github.com/pulumi/pulumi/pull/6896)
  [#7078](https://github.com/pulumi/pulumi/pull/7078)
  [#7079](https://github.com/pulumi/pulumi/pull/7079)
  [#7080](https://github.com/pulumi/pulumi/pull/7080)

- [sdk/nodejs|python] - Add GetSchema support to providers
  [#6892](https://github.com/pulumi/pulumi/pull/6892)

- [auto/dotnet] - Provide PulumiFn implementation that allows runtime stack type
  [#6910](https://github.com/pulumi/pulumi/pull/6910)

- [auto/go] - Provide GetPermalink for all results
  [#6875](https://github.com/pulumi/pulumi/pull/6875)

### Bug Fixes

- [sdk/python] Fix relative `runtime:options:virtualenv` path resolution to ignore `main` project attribute
  [#6966](https://github.com/pulumi/pulumi/pull/6966)

- [auto/dotnet] - Disable Language Server Host logging and checking appsettings.json config
  [#7023](https://github.com/pulumi/pulumi/pull/7023)

- [auto/python] - Export missing `ProjectBackend` type
  [#6984](https://github.com/pulumi/pulumi/pull/6984)

- [sdk/nodejs] - Fix noisy errors.
  [#6995](https://github.com/pulumi/pulumi/pull/6995)

- Config: Avoid emitting integers in objects using exponential notation.
  [#7005](https://github.com/pulumi/pulumi/pull/7005)

- [codegen/python] - Fix issue with lazy_import affecting pulumi-eks
  [#7024](https://github.com/pulumi/pulumi/pull/7024)

- Ensure that all outstanding asynchronous work is awaited before returning from a .NET
  Pulumi program.
  [#6993](https://github.com/pulumi/pulumi/pull/6993)

- Config: Avoid emitting integers in objects using exponential notation.
  [#7005](https://github.com/pulumi/pulumi/pull/7005)

- Build: Add vs code dev container
  [#7052](https://github.com/pulumi/pulumi/pull/7052)

- Ensure that all outstanding asynchronous work is awaited before returning from a Go
  Pulumi program. Note that this may require changes to programs that use the
  `pulumi.NewOutput` API.
  [#6983](https://github.com/pulumi/pulumi/pull/6983)

## 3.2.1 (2021-05-06)

### Bug Fixes

- [cli] Fix a regression caused by [#6893](https://github.com/pulumi/pulumi/pull/6893) that stopped stacks created
  with empty passphrases from completing successful pulumi commands when loading the passphrase secrets provider.
  [#6976](https://github.com/pulumi/pulumi/pull/6976)

## 3.2.0 (2021-05-05)

### Enhancements

- [auto/go] - Provide GetPermalink for all results
  [#6875](https://github.com/pulumi/pulumi/pull/6875)

- [automation/*] Add support for getting stack outputs using Workspace
  [#6859](https://github.com/pulumi/pulumi/pull/6859)

- [automation/*] Optionally skip Automation API version check
  [#6882](https://github.com/pulumi/pulumi/pull/6882)
  The version check can be skipped by passing a non-empty value to the `PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK` environment variable.

- [auto/go,nodejs] Add UserAgent to update/pre/refresh/destroy options.
  [#6935](https://github.com/pulumi/pulumi/pull/6935)

### Bug Fixes

- [cli] Return an appropriate error when a user has not set `PULUMI_CONFIG_PASSPHRASE` nor `PULUMI_CONFIG_PASSPHRASE_FILE`
  when trying to access the Passphrase Secrets Manager
  [#6893](https://github.com/pulumi/pulumi/pull/6893)

- [cli] Prevent against panic when using a ResourceReference as a program output
  [#6962](https://github.com/pulumi/pulumi/pull/6962)

- [sdk/python] - Fix bug in MockResourceArgs.
  [#6863](https://github.com/pulumi/pulumi/pull/6863)

- [sdk/python] Address issues when using resource subclasses.
  [#6890](https://github.com/pulumi/pulumi/pull/6890)

- [sdk/python] Fix type-related regression on Python 3.6.
  [#6942](https://github.com/pulumi/pulumi/pull/6942)

- [sdk/python] Don't error when a dict input value has a mismatched type annotation.
  [#6949](https://github.com/pulumi/pulumi/pull/6949)

- [automation/dotnet] Fix EventLogWatcher failing to read events after an exception was thrown
  [#6821](https://github.com/pulumi/pulumi/pull/6821)

- [automation/dotnet] Use stackName in ImportStack
  [#6858](https://github.com/pulumi/pulumi/pull/6858)

- [automation/go] Improve autoError message formatting
  [#6924](https://github.com/pulumi/pulumi/pull/6924)

### Misc.

- [auto/dotnet] Bump YamlDotNet to 11.1.1
  [#6915](https://github.com/pulumi/pulumi/pull/6915)

- [sdk/dotnet] Enable deterministic builds
  [#6917](https://github.com/pulumi/pulumi/pull/6917)

- [auto/*] - Bump minimum version to v3.1.0.
  [#6852](https://github.com/pulumi/pulumi/pull/6852)

## 3.1.0 (2021-04-22)

### Breaking Changes

Please note, the following 2 breaking changes were included in our [3.0 changlog](https://www.pulumi.com/docs/get-started/install/migrating-3.0/#updated-cli-behavior-in-pulumi-30)
Unfortunately, the initial release did not include that change. We apologize for any confusion or inconvenience this may have included the addressed behaviour.

- [cli] Standardize stack select behavior to ensure that passing `--stack` does not make that the current stack.
  [#6840](https://github.com/pulumi/pulumi/pull/6840)

- [cli] Set pagination defaults for `pulumi stack history` to 10 entries.
  [#6841](https://github.com/pulumi/pulumi/pull/6841)

### Enhancements

- [sdk/nodejs] Handle providers for RegisterResourceRequest
  [#6795](https://github.com/pulumi/pulumi/pull/6795)

- [automation/dotnet] Remove dependency on Gprc.Tools for F# / Paket compatibility
  [#6793](https://github.com/pulumi/pulumi/pull/6793)

### Bug Fixes

- [codegen] Fix codegen for types that are used by both resources and functions.
  [#6811](https://github.com/pulumi/pulumi/pull/6811)

- [sdk/python] Fix bug in `get_resource_module` affecting resource hydration.
  [#6833](https://github.com/pulumi/pulumi/pull/6833)

- [automation/python] Fix bug in UpdateSummary deserialization for nested config values.
  [#6838](https://github.com/pulumi/pulumi/pull/6838)


## 3.0.0 (2021-04-19)

### Breaking Changes

- [sdk/cli] Bump version of Pulumi CLI and SDK to v3
  [#6554](https://github.com/pulumi/pulumi/pull/6554)

- Dropped support for NodeJS < v11.x

- [CLI] Standardize the `--stack` flag to *not* set the stack as current (i.e. setStack=false) across CLI commands.
  [#6300](https://github.com/pulumi/pulumi/pull/6300)

- [CLI] Set pagination defaults for `pulumi stack history` to 10 entries.
  [#6739](https://github.com/pulumi/pulumi/pull/6739)

- [CLI] Remove `pulumi history` command. This was previously deprecated and replaced by `pulumi stack history`
  [#6724](https://github.com/pulumi/pulumi/pull/6724)

- [sdk/*] Refactor Mocks newResource and call to accept an argument struct for future extensibility rather than individual args
  [#6672](https://github.com/pulumi/pulumi/pull/6672)

- [sdk/nodejs] Enable nodejs dynamic provider caching by default on program side.
  [#6704](https://github.com/pulumi/pulumi/pull/6704)

- [sdk/python] Improved dict key translation support (3.0-based providers will opt-in to the improved behavior)
  [#6695](https://github.com/pulumi/pulumi/pull/6695)

- [sdk/python] Allow using Python to build resource providers for multi-lang components.
  [#6715](https://github.com/pulumi/pulumi/pull/6715)

- [sdk/go] Simplify `Apply` method options to reduce binary size
  [#6607](https://github.com/pulumi/pulumi/pull/6607)

- [Automation/*] All operations use `--stack` to specify the stack instead of running `select stack` before the operation.
  [#6300](https://github.com/pulumi/pulumi/pull/6300)

- [Automation/go] Moving go automation API package from sdk/v2/go/x/auto -> sdk/v2/go/auto
  [#6518](https://github.com/pulumi/pulumi/pull/6518)

- [Automation/nodejs] Moving NodeJS automation API package from sdk/nodejs/x/automation -> sdk/nodejs/automation
  [#6518](https://github.com/pulumi/pulumi/pull/6518)

- [Automation/python] Moving Python automation API package from pulumi.x.automation -> pulumi.automation
  [#6518](https://github.com/pulumi/pulumi/pull/6518)

- [Automation/go] Moving go automation API package from sdk/v2/go/x/auto -> sdk/v2/go/auto
  [#6518](https://github.com/pulumi/pulumi/pull/6518)


### Enhancements

- [sdk/nodejs] Add support for multiple V8 VM contexts in closure serialization.
  [#6648](https://github.com/pulumi/pulumi/pull/6648)

- [sdk] Handle providers for RegisterResourceRequest
  [#6771](https://github.com/pulumi/pulumi/pull/6771)
  [#6781](https://github.com/pulumi/pulumi/pull/6781)
  [#6786](https://github.com/pulumi/pulumi/pull/6786)

- [sdk/go] Support defining remote components in Go.
  [#6403](https://github.com/pulumi/pulumi/pull/6403)


### Bug Fixes

- [CLI] Clean the template cache if the repo remote has changed.
  [#6784](https://github.com/pulumi/pulumi/pull/6784)


## 2.25.2 (2021-04-17)

### Bug Fixes

- [cli] Fix a bug that prevented copying checkpoint files when using Azure Blob Storage
  as the backend provider. [#6794](https://github.com/pulumi/pulumi/pull/6794)

## 2.25.1 (2021-04-15)

### Bug Fixes

- [automation/python] - Fix serialization bug in `StackSettings`
  [#6776](https://github.com/pulumi/pulumi/pull/6776)

## 2.25.0 (2021-04-14)

### Breaking

- [automation/dotnet] Rename (Get,Set,Remove)Config(Value)
  [#6731](https://github.com/pulumi/pulumi/pull/6731)

  The following methods on Workspace and WorkspaceStack classes have
  been renamed. Please update your code (before -> after):

  * GetConfigValue -> GetConfig
  * SetConfigValue -> SetConfig
  * RemoveConfigValue -> RemoveConfig
  * GetConfig -> GetAllConfig
  * SetConfig -> SetAllConfig
  * RemoveConfig -> RemoveAllConfig

  This change was made to align with the other Pulumi language SDKs.

### Improvements

- [cli] Add option to print absolute rather than relative dates in stack history
  [#6742](https://github.com/pulumi/pulumi/pull/6742)  

  Example:
  ```bash
  pulumi stack history --full-dates
  ```

- [cli] Enable absolute and relative parent paths for pulumi main
  [#6734](https://github.com/pulumi/pulumi/pull/6734)

- [sdk/dotnet] Thread-safe concurrency-friendly global state
  [#6139](https://github.com/pulumi/pulumi/pull/6139)
  
- [tooling] Update pulumi python docker image to python 3.9
  [#6706](https://github.com/pulumi/pulumi/pull/6706)

- [sdk/nodejs] Add program side caching for dynamic provider serialization behind env var
  [#6673](https://github.com/pulumi/pulumi/pull/6673)

- [sdk/nodejs] Allow prompt values in `construct` for multi-lang components.
  [#6522](https://github.com/pulumi/pulumi/pull/6522)

- [automation/dotnet] Allow null environment variables
  [#6687](https://github.com/pulumi/pulumi/pull/6687)

- [automation/dotnet] Expose WorkspaceStack.GetOutputsAsync
  [#6699](https://github.com/pulumi/pulumi/pull/6699)

  Example:
  ```csharp
  var stack = await WorkspaceStack.CreateAsync(stackName, workspace);
  await stack.SetConfigAsync(config);
  var initialOutputs = await stack.GetOutputsAsync();
  ```

- [automation/dotnet] Implement (Import,Export)StackAsync methods on LocalWorkspace and WorkspaceStack and expose StackDeployment helper class.
  [#6728](https://github.com/pulumi/pulumi/pull/6728)

  Example:
  ```csharp
  var stack = await WorkspaceStack.CreateAsync(stackName, workspace);
  var upResult = await stack.UpAsync();
  deployment = await workspace.ExportStackAsync(stackName);
  ```
  
- [automation/dotnet] Implement CancelAsync method on WorkspaceStack
  [#6729](https://github.com/pulumi/pulumi/pull/6729)

  Example:
  ```csharp
  var stack = await WorkspaceStack.CreateAsync(stackName, workspace);
  var cancelTask = stack.CancelAsync();
  ```

- [automation/python] - Expose structured logging for Stack.up/preview/refresh/destroy.
  [#6527](https://github.com/pulumi/pulumi/pull/6527)

  You can now pass in an `on_event` callback function as a keyword arg to `up`, `preview`, `refresh`
  and `destroy` to process streaming json events defined in `automation/events.py`

  Example:
  ```python
  stack.up(on_event=print)
  ```

### Bug Fixes

- [sdk/go] Fix wrongly named Go modules
  [#6775](https://github.com/pulumi/pulumi/issues/6775)

- [cli] Handle non-existent creds file in `pulumi logout --all`
  [#6741](https://github.com/pulumi/pulumi/pull/6741)

- [automation/nodejs] Do not run the promise leak checker if an inline program has errored.
  [#6758](https://github.com/pulumi/pulumi/pull/6758)

- [sdk/nodejs] Explicitly create event log file for NodeJS Automation API.
  [#6730](https://github.com/pulumi/pulumi/pull/6730)

- [sdk/nodejs] Fix error handling for failed logging statements
  [#6714](https://github.com/pulumi/pulumi/pull/6714)

- [sdk/nodejs] Fix `Construct` to wait for child resources of a multi-lang components to be created.
  [#6452](https://github.com/pulumi/pulumi/pull/6452)

- [sdk/python] Fix serialization bug if output contains 'items' property.
  [#6701](https://github.com/pulumi/pulumi/pull/6701)

- [automation] Set default value for 'main' for inline programs to support relative paths, assets, and closure serialization.
  [#6743](https://github.com/pulumi/pulumi/pull/6743)
  
- [automation/dotnet] Environment variable value type is now nullable.
  [#6520](https://github.com/pulumi/pulumi/pull/6520)

- [automation/dotnet] Fix GetConfigValueAsync failing to deserialize
  [#6698](https://github.com/pulumi/pulumi/pull/6698)

- [automation] Fix (de)serialization of StackSettings in .NET, Node, and Python.
  [#6752](https://github.com/pulumi/pulumi/pull/6752)
  [#6754](https://github.com/pulumi/pulumi/pull/6754)
  [#6749](https://github.com/pulumi/pulumi/pull/6749)


## 2.24.1 (2021-04-01)

### Bug Fixes

- [cli] Revert the swapping out of the YAML parser library
  [#6681](https://github.com/pulumi/pulumi/pull/6681)

- [automation/go,python,nodejs] Respect pre-existing Pulumi.yaml for inline programs.
  [#6655](https://github.com/pulumi/pulumi/pull/6655)

## 2.24.0 (2021-03-31)

### Improvements

- [sdk/nodejs] Add provider side caching for dynamic provider deserialization
  [#6657](https://github.com/pulumi/pulumi/pull/6657)

- [automation/dotnet] Expose structured logging
  [#6572](https://github.com/pulumi/pulumi/pull/6572)

- [cli] Support full fidelity YAML round-tripping
  - Strip Byte-order Mark (BOM) from YAML configs during load. - [#6636](https://github.com/pulumi/pulumi/pull/6636)
  - Swap out YAML parser library - [#6642](https://github.com/pulumi/pulumi/pull/6642)

- [sdk/python] Ensure all async tasks are awaited prior to exit.
  [#6606](https://github.com/pulumi/pulumi/pull/6606)

### Bug Fixes

- [sdk/nodejs] Fix error propagation in registerResource and other resource methods.
  [#6644](https://github.com/pulumi/pulumi/pull/6644)

- [automation/python] Fix passing of additional environment variables.
  [#6639](https://github.com/pulumi/pulumi/pull/6639)

- [sdk/python] Make exceptions raised by calls to provider functions (e.g. data sources) catchable.
  [#6504](https://github.com/pulumi/pulumi/pull/6504)

- [automation/go,python,nodejs] Respect pre-existing Pulumi.yaml for inline programs.
  [#6655](https://github.com/pulumi/pulumi/pull/6655)

## 2.23.2 (2021-03-25)

### Improvements

- [cli] Improve diff displays during `pulumi refresh`
  [#6568](https://github.com/pulumi/pulumi/pull/6568)

- [sdk/go] Cache loaded configuration files.
  [#6576](https://github.com/pulumi/pulumi/pull/6576)

- [sdk/nodejs] Allow `Mocks::newResource` to determine whether the created resource is a `CustomResource`.
  [#6551](https://github.com/pulumi/pulumi/pull/6551)

- [automation/*] Implement minimum version checking and add:
  - Go: `LocalWorkspace.PulumiVersion()` - [#6577](https://github.com/pulumi/pulumi/pull/6577)
  - Nodejs: `LocalWorkspace.pulumiVersion` - [#6580](https://github.com/pulumi/pulumi/pull/6580)
  - Python: `LocalWorkspace.pulumi_version` - [#6589](https://github.com/pulumi/pulumi/pull/6589)
  - Dotnet: `LocalWorkspace.PulumiVersion` - [#6590](https://github.com/pulumi/pulumi/pull/6590)

### Bug Fixes

- [sdk/python] Fix automatic venv creation
  [#6599](https://github.com/pulumi/pulumi/pull/6599)

- [automation/python] Fix Settings file save
  [#6605](https://github.com/pulumi/pulumi/pull/6605)

- [sdk/dotnet] Remove MaybeNull from Output/Input.Create to avoid spurious warnings
  [#6600](https://github.com/pulumi/pulumi/pull/6600)

## 2.23.1 (2021-03-17)

### Bug Fixes

- [cli] Fix a bug where a version wasn't passed to go install commands as part of `make brew` installs from homebrew
  [#6566](https://github.com/pulumi/pulumi/pull/6566)

## 2.23.0 (2021-03-17)

### Breaking

- [automation/go] - Expose structured logging for Stack.Up/Preview/Refresh/Destroy.
  [#6436](https://github.com/pulumi/pulumi/pull/6436)

This change is marked breaking because it changes the shape of the `PreviewResult` struct.

**Before**

```go
type PreviewResult struct {
  Steps         []PreviewStep  `json:"steps"`
  ChangeSummary map[string]int `json:"changeSummary"`
}
```

**After**

```go
type PreviewResult struct {
  StdOut        string
  StdErr        string
  ChangeSummary map[apitype.OpType]int
}
```

- [automation/dotnet] Add ability to capture stderr
  [#6513](https://github.com/pulumi/pulumi/pull/6513)

This change is marked breaking because it also renames `OnOutput` to `OnStandardOutput`.

### Improvements

- [sdk/go] Add helpers to convert raw Go maps and arrays to Pulumi `Map` and `Array` inputs.
  [#6337](https://github.com/pulumi/pulumi/pull/6337)

- [sdk/go] Return zero values instead of panicing in `Index` and `Elem` methods.
  [#6338](https://github.com/pulumi/pulumi/pull/6338)

- [sdk/go] Support multiple folders in GOPATH.
  [#6228](https://github.com/pulumi/pulumi/pull/6228

- [cli] Add ability to download arm64 provider plugins
  [#6492](https://github.com/pulumi/pulumi/pull/6492)

- [build] Updating Pulumi to use Go 1.16
  [#6470](https://github.com/pulumi/pulumi/pull/6470)

- [build] Adding a Pulumi arm64 binary for use on new macOS hardware.
  Please note that `pulumi watch` will not be supported on darwin/arm64 builds.
  [#6492](https://github.com/pulumi/pulumi/pull/6492)

- [automation/nodejs] - Expose structured logging for Stack.up/preview/refresh/destroy.
  [#6454](https://github.com/pulumi/pulumi/pull/6454)

- [automation/nodejs] - Add `onOutput` event handler to `PreviewOptions`.
  [#6507](https://github.com/pulumi/pulumi/pull/6507)

- [cli] Add locking support to the self-managed backends using the `PULUMI_SELF_MANAGED_STATE_LOCKING=1` environment variable.
  [#2697](https://github.com/pulumi/pulumi/pull/2697)

### Bug Fixes

- [sdk/python] Fix mocks issue when passing a resource more than once.
  [#6479](https://github.com/pulumi/pulumi/pull/6479)

- [automation/dotnet] Add ReadDiscard OperationType
  [#6493](https://github.com/pulumi/pulumi/pull/6493)

- [cli] Ensure the user has the correct access to the secrets manager before using it as part of
  `pulumi stack export --show-secrets`.
  [#6215](https://github.com/pulumi/pulumi/pull/6210)

- [sdk/go] Implement getResource in the mock monitor.
  [#5923](https://github.com/pulumi/pulumi/pull/5923)

## 2.22.0 (2021-03-03)

### Improvements

- [#6410](https://github.com/pulumi/pulumi/pull/6410) Add `diff` option to Automation API's `preview` and `up`

### Bug Fixes

- [automation/dotnet] - resolve issue with OnOutput delegate not being called properly during pulumi process execution.
  [#6435](https://github.com/pulumi/pulumi/pull/6435)

- [automation/python,nodejs,dotnet] - BREAKING - Remove `summary` property from `PreviewResult`.
  The `summary` property on `PreviewResult` returns a result that is always incorrect and is being removed.
  [#6405](https://github.com/pulumi/pulumi/pull/6405)

- [automation/python] - Fix Windows error caused by use of NamedTemporaryFile in automation api.
  [#6421](https://github.com/pulumi/pulumi/pull/6421)

- [sdk/nodejs] Serialize default parameters correctly. [#6397](https://github.com/pulumi/pulumi/pull/6397)

- [cli] Respect provider aliases while diffing resources.
  [#6453](https://github.com/pulumi/pulumi/pull/6453)

## 2.21.2 (2021-02-22)

### Improvements

- [cli] Disable permalinks to the update details page when using self-managed backends (S3, Azure, GCS). Should the user
  want to get permalinks when using a self backend, they can pass a flag:  
      `pulumi up --suppress-permalink false`.  
  Permalinks for these self-managed backends will be suppressed on `update`, `preview`, `destroy`, `import` and `refresh`
  operations.
  [#6251](https://github.com/pulumi/pulumi/pull/6251)

- [cli] Added commands `config set-all` and `config rm-all` to set and remove multiple configuration keys.
  [#6373](https://github.com/pulumi/pulumi/pull/6373)

- [automation/*] Consume `config set-all` and `config rm-all` from automation API.
  [#6388](https://github.com/pulumi/pulumi/pull/6388)

- [sdk/dotnet] C# Automation API.
  [#5761](https://github.com/pulumi/pulumi/pull/5761)

- [sdk/dotnet] F# API to specify stack options.
  [#5077](https://github.com/pulumi/pulumi/pull/5077)

### Bug Fixes

- [sdk/nodejs] Don't error when loading multiple copies of the same version of a Node.js
  component package. [#6387](https://github.com/pulumi/pulumi/pull/6387)

- [cli] Skip unnecessary state file writes to address performance regression introduced in 2.16.2.
  [#6396](https://github.com/pulumi/pulumi/pull/6396)

## 2.21.1 (2021-02-18)

### Bug Fixes

- [sdk/python] Fixed a change to `Output.all()` that raised an error if no inputs are passed in.
  [#6381](https://github.com/pulumi/pulumi/pull/6381)

## 2.21.0 (2021-02-17)

### Improvements

- [cli] Added pagination options to `pulumi stack history` [#6292](https://github.com/pulumi/pulumi/pull/6292)  
  This is used as follows:  
  `pulumi stack history --page-size=20 --page=1`

- [automation/*] Added pagination options for stack history in Automation API SDKs to improve
  performance of stack updates. [#6257](https://github.com/pulumi/pulumi/pull/6257)    
  This is used similar to the following example in go:  
```go
  func ExampleStack_History() {
	ctx := context.Background()
	stackName := FullyQualifiedStackName("org", "project", "stack")
	stack, _ := SelectStackLocalSource(ctx, stackName, filepath.Join(".", "program"))
	pageSize := 0
	page := 0
	hist, _ := stack.History(ctx, pageSize, page)
	fmt.Println(hist[0].StartTime)
  }
```

- [pkg/testing/integration] Changed the default behavior for Python test projects to use `UseAutomaticVirtualEnv` by
  default. `UsePipenv` is now the way to use pipenv with tests.
  [#6318](https://github.com/pulumi/pulumi/pull/6318)

### Bug Fixes

- [automation/go] Exposed the version in the UpdateSummary for use in understanding the version of a stack update
  [#6339](https://github.com/pulumi/pulumi/pull/6339)

- [cli] Changed the behavior for Python on Windows to look for `python` binary first instead of `python3`.
  [#6317](https://github.com/pulumi/pulumi/pull/6317)

- [sdk/python] Gracefully handle monitor shutdown in the python runtime without exiting the process.
  [#6249](https://github.com/pulumi/pulumi/pull/6249)

- [sdk/python] Fixed a bug in `contains_unknowns` where outputs with a property named "values" failed with a TypeError.
  [#6264](https://github.com/pulumi/pulumi/pull/6264)

- [sdk/python] Allowed keyword args in Output.all() to create a dict.
  [#6269](https://github.com/pulumi/pulumi/pull/6269)

- [sdk/python] Defined `__all__` in modules for better IDE autocomplete.
  [#6351](https://github.com/pulumi/pulumi/pull/6351)

- [automation/python] Fixed a bug in nested configuration parsing.
  [#6349](https://github.com/pulumi/pulumi/pull/6349)

## 2.20.0 (2021-02-03)

- [sdk/python] Fix `Output.from_input` to unwrap nested output values in input types (args classes), which addresses
  an issue that was preventing passing instances of args classes with nested output values to Provider resources.
  [#6221](https://github.com/pulumi/pulumi/pull/6221)

## 2.19.0 (2021-01-27)

- [sdk/nodejs] Always read and write NodeJS runtime options from the environment.
  [#6076](https://github.com/pulumi/pulumi/pull/6076)

- [sdk/go] Take a breaking change to remove unidiomatic numerical types and drastically improve build performance (binary size and compilation time).
  [#6143](https://github.com/pulumi/pulumi/pull/6143)

- [cli] Ensure `pulumi stack change-secrets-provider` allows rotating the key from hashivault to passphrase provider
  [#6210](https://github.com/pulumi/pulumi/pull/6210)

## 2.18.2 (2021-01-22)

- [CLI] Fix malformed resource value bug.
  [#6164](https://github.com/pulumi/pulumi/pull/6164)

- [sdk/dotnet] Fix `RegisterResourceOutputs` to serialize resources as resource references
  only when the monitor reports that resource references are supported.
  [#6172](https://github.com/pulumi/pulumi/pull/6172)

- [CLI] Avoid panic for diffs with invalid property paths.
  [#6159](https://github.com/pulumi/pulumi/pull/6159)

- Enable resource reference feature by default.
  [#6202](https://github.com/pulumi/pulumi/pull/6202)

## 2.18.1 (2021-01-21)

- Revert [#6125](https://github.com/pulumi/pulumi/pull/6125) as it caused a which introduced a bug with serializing resource IDs

## 2.18.0 (2021-01-20)

- [CLI] Add the ability to log out of all Pulumi backends at once.
  [#6101](https://github.com/pulumi/pulumi/pull/6101)

- [sdk/go] Added `pulumi.Unsecret` which will take an existing secret output and
  create a non-secret variant with an unwrapped secret value. Also adds,
  `pulumi.IsSecret` which will take an existing output and
  determine if an output has a secret within the output.
  [#6085](https://github.com/pulumi/pulumi/pull/6085)

## 2.17.2 (2021-01-14)

- .NET: Allow `IMock.NewResourceAsync` to return a null ID for component resources.
  Note that this may require mocks written in C# to be updated to account for the
  change in nullability.
  [#6104](https://github.com/pulumi/pulumi/pull/6104)

- [automation/go] Add debug logging settings for common automation API operations
  [#6095](https://github.com/pulumi/pulumi/pull/6095)

- [automation/go] Set DryRun on previews so unknowns are identified correctly.
  [#6099](https://github.com/pulumi/pulumi/pull/6099)

- [sdk/python] Fix python 3.6 support by removing annotations import.
  [#6109](https://github.com/pulumi/pulumi/pull/6109)

- [sdk/nodejs] Added `pulumi.unsecret` which will take an existing secret output and
  create a non-secret variant with an unwrapped secret value. Also adds,
  `pulumi.isSecret` which will take an existing output and
  determine if an output has a secret within the output.
  [#6086](https://github.com/pulumi/pulumi/pull/6086)

- [sdk/python] Added `pulumi.unsecret` which will take an existing secret output and
  create a non-secret variant with an unwrapped secret value. Also adds,
  `pulumi.is_secret` which will take an existing output and
  determine if an output has a secret within the output.
  [#6111](https://github.com/pulumi/pulumi/pull/6111)

## 2.17.1 (2021-01-13)

- Fix an issue with go sdk generation where optional strict enum values
  could not be omitted. Note - this is a breaking change to go sdk's enum
  values. However we currently only support strict enums in the azure-nextgen provider's schema.
  [#6069](https://github.com/pulumi/pulumi/pull/6069)

- Fix an issue where python debug messages print unexpectedly.
  [#6967](https://github.com/pulumi/pulumi/pull/6067)

- [CLI] Add `version` to the stack history output to be able to
  correlate events back to the Pulumi SaaS
  [#6063](https://github.com/pulumi/pulumi/pull/6063)

- Fix a typo in the unit testing mocks to get the outputs
  while registering them
  [#6040](https://github.com/pulumi/pulumi/pull/6040)

- [sdk/dotnet] Moved urn value retrieval into if statement
  for MockMonitor
  [#6081](https://github.com/pulumi/pulumi/pull/6081)

- [sdk/dotnet] Added `Pulumi.Output.Unsecret` which will
  take an existing secret output and
  create a non-secret variant with an unwrapped secret value.
  [#6092](https://github.com/pulumi/pulumi/pull/6092)

- [sdk/dotnet] Added `Pulumi.Output.IsSecretAsync` which will
  take an existing output and
  determine if an output has a secret within the output.
  [#6092](https://github.com/pulumi/pulumi/pull/6092)

- [sdk/dotnet] Fix looking up empty version in
  `ResourcePackages.TryGetResourceType`.
  [#6084](https://github.com/pulumi/pulumi/pull/6084)

- Python Automation API.
  [#5979](https://github.com/pulumi/pulumi/pull/5979)

- Support recovery workflow (import/export/cancel) in Python Automation API.
  [#6037](https://github.com/pulumi/pulumi/pull/6037)

## 2.17.0 (2021-01-06)

- Respect the `version` resource option for provider resources.
  [#6055](https://github.com/pulumi/pulumi/pull/6055)

- Allow `serializeFunction` to capture secrets.
  [#6013](https://github.com/pulumi/pulumi/pull/6013)

- [CLI] Allow `pulumi console` to accept a stack name
  [#6031](https://github.com/pulumi/pulumi/pull/6031)

- Support recovery workflow (import/export/cancel) in NodeJS Automation API.
  [#6038](https://github.com/pulumi/pulumi/pull/6038)

- [CLI] Add a confirmation prompt when using `pulumi policy rm`
  [#6034](https://github.com/pulumi/pulumi/pull/6034)

- [CLI] Ensure errors with the Pulumi credentials file
  give the user some information on how to resolve the problem
  [#6044](https://github.com/pulumi/pulumi/pull/6044)

- [sdk/go] Support maps in Invoke outputs and Read inputs
  [#6014](https://github.com/pulumi/pulumi/pull/6014)

## 2.16.2 (2020-12-23)

- Fix a bug in the core engine that could cause previews to fail if a resource with changes had
  unknown output property values.
  [#6006](https://github.com/pulumi/pulumi/pull/6006)

## 2.16.1 (2020-12-22)

- Fix a panic due to unsafe concurrent map access.
  [#5995](https://github.com/pulumi/pulumi/pull/5995)

- Fix regression in `venv` creation for python policy packs.
  [#5992](https://github.com/pulumi/pulumi/pull/5992)

## 2.16.0 (2020-12-21)

- Do not read plugins and policy packs into memory prior to extraction, as doing so can exhaust
  the available memory on lower-end systems.
  [#5983](https://github.com/pulumi/pulumi/pull/5983)

- Fix a bug in the core engine where deleting/renaming a resource would panic on update + refresh.
  [#5980](https://github.com/pulumi/pulumi/pull/5980)

- Fix a bug in the core engine that caused `ignoreChanges` to fail for resources being imported.
  [#5976](https://github.com/pulumi/pulumi/pull/5976)

- Fix a bug in the core engine that could cause resources references to marshal improperly
  during preview.
  [#5960](https://github.com/pulumi/pulumi/pull/5960)

- [sdk/dotnet] Add collection initializers for smooth support of Union<T, U> as element type
  [#5938](https://github.com/pulumi/pulumi/pull/5938)

- Fix a bug in the core engine where ComponentResource state would be accessed before initialization.
  [#5949](https://github.com/pulumi/pulumi/pull/5949)

- Prevent a panic by not attempting to show progress for zero width/height terminals.
  [#5957](https://github.com/pulumi/pulumi/issues/5957)

## 2.15.6 (2020-12-12)

- Fix a bug in the Go SDK that could result in dropped resource dependencies.
  [#5930](https://github.com/pulumi/pulumi/pull/5930)

- Temporarily disable resource ref feature.
  [#5932](https://github.com/pulumi/pulumi/pull/5932)

## 2.15.5 (2020-12-11)

- Re-apply fix for running multiple `pulumi` processes concurrently.
  [#5893](https://github.com/pulumi/pulumi/issues/5893)

- [cli] Prevent a panic when using `pulumi import` with local filesystems
  [#5906](https://github.com/pulumi/pulumi/issues/5906)

- [sdk/nodejs] Fix issue that would cause unit tests using mocks to fail with unhandled errors when
  a resource references another resources that's been registered with `registerResourceModule`.
  [#5914](https://github.com/pulumi/pulumi/pull/5914)

- Enable resource reference feature by default.
  [#5905](https://github.com/pulumi/pulumi/pull/5905)

- [codegen/go] Fix Input/Output methods for Go resources.
  [#5916](https://github.com/pulumi/pulumi/pull/5916)

- [sdk/python] Implement getResource in the mock monitor.
  [#5919](https://github.com/pulumi/pulumi/pull/5919)

- [sdk/dotnet] Implement getResource in the mock monitor and fix some issues around
  deserializing resources.
  [#5921](https://github.com/pulumi/pulumi/pull/5921)

## 2.15.4 (2020-12-08)

- Fix a problem where `pulumi import` could panic on an import error due to missing error message.
  [#5884](https://github.com/pulumi/pulumi/pull/5884)
- Correct the system name detected for Jenkins CI. [#5891](https://github.com/pulumi/pulumi/pull/5891)

- Fix python execution for users running Python installed through the Windows App Store
  on Windows 10 [#5874](https://github.com/pulumi/pulumi/pull/5874)

## 2.15.3 (2020-12-07)

- Fix errors when running `pulumi` in Windows-based CI environments.
  [#5879](https://github.com/pulumi/pulumi/issues/5879)

## 2.15.2 (2020-12-07)

- Fix a problem where `pulumi import` could panic on importing arrays and sets, due to
  incorrect array resizing logic. [#5872](https://github.com/pulumi/pulumi/pull/5872).

## 2.15.1 (2020-12-04)

- Address potential issues when running multiple `pulumi` processes concurrently.
  [#5857](https://github.com/pulumi/pulumi/pull/5857)

- Automatically install missing Python dependencies.
  [#5787](https://github.com/pulumi/pulumi/pull/5787)

- [cli] Ensure `pulumi stack change-secrets-provider` allows rotating the key for a passphrase provider
  [#5865](https://github.com/pulumi/pulumi/pull/5865/)

## 2.15.0 (2020-12-02)

- [sdk/python] Add deserialization support for enums.
  [#5615](https://github.com/pulumi/pulumi/pull/5615)

- Correctly rename `Pulumi.*.yaml` stack files during a rename that includes an
  organization in its name [#5812](https://github.com/pulumi/pulumi/pull/5812).

- Respect `PULUMI_PYTHON_CMD` in scripts.
  [#5782](https://github.com/pulumi/pulumi/pull/5782)

- Add `PULUMI_BACKEND_URL` environment variable to configure the state backend.
  [#5789](https://github.com/pulumi/pulumi/pull/5789)

- [sdk/dotnet] Add support for dependency injection into TStack instance by adding an overload to `Deployment.RunAsync`. The overload accepts an `IServiceProvider` that is used to create the instance of TStack. Also added a new method `Deployment.TestWithServiceProviderAsync` for testing stacks that use dependency injection.
  [#5723](https://github.com/pulumi/pulumi/pull/5723/)

- [cli] Ensure `pulumi stack change-secrets-provider` allows rotating the key in Azure KeyVault
  [#5842](https://github.com/pulumi/pulumi/pull/5842/)

## 2.14.0 (2020-11-18)

- Propagate secretness of provider configuration through to the statefile. This ensures
  that any configuration values marked as secret (i.e. values set with
  `pulumi config set --secret`) that are used as inputs to providers are encrypted
  before they are stored.
  [#5742](https://github.com/pulumi/pulumi/pull/5742)

- Fix a bug that could prevent `pulumi import` from succeeding.
  [#5730](https://github.com/pulumi/pulumi/pull/5730)

- [Docs] Add support for the generation of Import documentation in the resource docs.
  This documentation will only be available if the resource is importable.
  [#5667](https://github.com/pulumi/pulumi/pull/5667)

- [codegen/go] Add support for ResourceType and isComponent to enable multi-language
  components in Go. This change also generates Input/Output types for all resources
  in downstream Go SDKs.
  [#5497](https://github.com/pulumi/pulumi/pull/5497)

- Support python 3.9 on Windows.
  [#5739](https://github.com/pulumi/pulumi/pull/5739)

- `pulumi-language-go` and `pulumi new` now explicitly requires Go 1.14.0 or greater.
  [#5741](https://github.com/pulumi/pulumi/pull/5741)

- Update .NET `Grpc` libraries to 2.33.1 and `Protobuf` to 3.13.0 (forked to increase
  the recursion limit) [#5757](https://github.com/pulumi/pulumi/pull/5757)

- Fix plugin install failures on Windows.
  [#5759](https://github.com/pulumi/pulumi/pull/5759)

- .NET: Report plugin install errors during `pulumi new`.
  [#5760](https://github.com/pulumi/pulumi/pull/5760)

- Correct error message on KeyNotFoundException against StackReference.
  [#5740](https://github.com/pulumi/pulumi/pull/5740)

- [cli] Small UX change on the policy violations output to render as `type: name`
  [#5773](https://github.com/pulumi/pulumi/pull/5773)

## 2.13.2 (2020-11-06)

- Fix a bug that was causing errors when (de)serializing custom resources.
  [#5709](https://github.com/pulumi/pulumi/pull/5709)

## 2.13.1 (2020-11-06)

- [cli] Ensure `pulumi history` annotes when secrets are unable to be decrypted
  [#5701](https://github.com/pulumi/pulumi/pull/5701)

- Fix a bug in the Python SDK that caused incompatibilities with versions of the CLI prior to
  2.13.0.
  [#5702](https://github.com/pulumi/pulumi/pull/5702)

## 2.13.0 (2020-11-04)

- Add internal scaffolding for using cross-language components from Go.
  [#5558](https://github.com/pulumi/pulumi/pull/5558)

- Support python 3.9.
  [#5669](https://github.com/pulumi/pulumi/pull/5669)

- [cli] Ensure that the CLI doesn't panic when using pulumi watch and using ComponentResources with non-standard naming
  [#5675](https://github.com/pulumi/pulumi/pull/5675)

- [cli] Ensure that the CLI doesn't panic when trying to assemble a graph on a stack that has no snapshot available
  [#5678](https://github.com/pulumi/pulumi/pull/5678)

- Add boolean values to Go SDK
  [#5687](https://github.com/pulumi/pulumi/pull/5687)

## 2.12.1 (2020-10-23)

- [cli] Ensure that the CLI doesn't panic when using pulumi watch and policies are enabled
  [#5569](https://github.com/pulumi/pulumi/pull/5569)

- [cli] Ensure that the CLI doesn't panic when using the JSON output as part of previews
  and policies are enabled
  [#5610](https://github.com/pulumi/pulumi/pull/5610)


## 2.12.0 (2020-10-14)

- NodeJS Automation API.
  [#5347](https://github.com/pulumi/pulumi/pull/5347)

- Improve the accuracy of previews by allowing providers to participate in determining what
  the impact of a change will be on output properties. Previously, Pulumi previews
  conservatively assumed that any output-only properties changed their values when an update
  occurred. For many properties, this was guaranteed to not be the case (because those
  properties are immutable, for example), and by suggesting the value might change, this could
  lead to the preview suggesting additional transitive updates of even replaces that would not
  actually happen during an update. Pulumi now allows the provider to specify the details of
  what properties will change during a preview, allowing them to expose more accurate
  provider-specific knowledge. This change is less conservative than the previous behavior,
  and so in case it causes preview results which are not deemed correct in some case - the
  `PULUMI_DISABLE_PROVIDER_PREVIEW` flag can be set to a truthy value (e.g. `1`) to enable the
  previous and more conservative behavior for previews.
  [#5443](https://github.com/pulumi/pulumi/pull/5443).

- Add an import command to the Pulumi CLI. This command can be used to import existing resources
  into a Pulumi stack.
  [#4765](https://github.com/pulumi/pulumi/pull/4765)

- [cli] Remove eternal loop if a configured passphrase is invalid.
  [#5507](https://github.com/pulumi/pulumi/pull/5507)

- Correctly validate project names during 'pulumi new'
  [#5504](https://github.com/pulumi/pulumi/pull/5504)

- Fixing gzip compression for alternative backends.
  [#5484](https://github.com/pulumi/pulumi/pull/5484)

- Add internal scaffolding for using cross-language components from .NET.
  [#5485](https://github.com/pulumi/pulumi/pull/5485)

- Support self-contained executables as binary option for .NET programs.
  [#5519](https://github.com/pulumi/pulumi/pull/5519)

- [cli] Ensure old secret provider variables are cleaned up when changing between secret providers
  [#5545](https://github.com/pulumi/pulumi/pull/5545)

- [cli] Respect logging verbosity as part of pulumi plugin install commands
  [#5549](https://github.com/pulumi/pulumi/pull/5549)

- [cli] Accept `-f` as a shorthand for `--skip-preview` on `pulumi up`, `pulumi refresh` and `pulumi destroy` operations
  [#5556](https://github.com/pulumi/pulumi/pull/5556)

- [cli] Validate cloudUrl formats before `pulumi login` and throw an error if incorrect format specified
  [#5550](https://github.com/pulumi/pulumi/pull/5545)

- [automation api] Add support for passing a private ssh key for git authentication that doesn't rely on a file path
  [#5557](https://github.com/pulumi/pulumi/pull/5557)

- [cli] Improve user experience when pulumi plugin rm --all finds no plugins
  to remove. The previous behaviour was an error and should not be so.
  [#5547](https://github.com/pulumi/pulumi/pull/5547)

- [sdk/python] Fix ResourceOptions annotations and doc strings.
  [#5559](https://github.com/pulumi/pulumi/pull/5559)

- [sdk/dotnet] Fix HashSet concurrency issue.
  [#5563](https://github.com/pulumi/pulumi/pull/5563)

## 2.11.2 (2020-10-01)

- feat(autoapi): expose EnvVars LocalWorkspaceOption to set in ctor
  [#5499](https://github.com/pulumi/pulumi/pull/5499)

- [sdk/python] Fix secret regression: ensure unwrapped secrets during deserialization
  are rewrapped before being returned.
  [#5496](https://github.com/pulumi/pulumi/pull/5496)

## 2.11.1 (2020-09-30)

- Add internal scaffolding for using cross-language components from Python.
  [#5375](https://github.com/pulumi/pulumi/pull/5375)

## 2.11.0 (2020-09-30)

- Do not oversimplify types for display when running an update or preview.
  [#5440](https://github.com/pulumi/pulumi/pull/5440)

- Pulumi Windows CLI now uploads all VCS information to console
  (fixes [#5014](https://github.com/pulumi/pulumi/issues/5014))
  [#5406](https://github.com/pulumi/pulumi/pull/5406)

- .NET SDK: Support `Output<object>` for resource output properties
  (fixes [#5446](https://github.com/pulumi/pulumi/issues/5446))
  [#5465](https://github.com/pulumi/pulumi/pull/5465)

## 2.10.2 (2020-09-21)

- [sdk/go] Add missing Version field to invokeOptions
  [#5401](https://github.com/pulumi/pulumi/pull/5401)

- Add `pulumi console` command which opens the currently selected stack in the Pulumi console.
  [#5368](https://github.com/pulumi/pulumi/pull/5368)

- Python SDK: Cast numbers intended to be integers to `int`.
  [#5419](https://github.com/pulumi/pulumi/pull/5419)

## 2.10.1 (2020-09-16)

- feat(autoapi): add GetPermalink for operation result
  [#5363](https://github.com/pulumi/pulumi/pull/5363)

- Relax stack name validations for Automation API [#5337](https://github.com/pulumi/pulumi/pull/5337)

- Allow Pulumi to read a passphrase file, via `PULUMI_CONFIG_PASSPHRASE_FILE` to interact
  with the passphrase secrets provider. Pulumi will first try and use the `PULUMI_CONFIG_PASSPHRASE`
  to get the passphrase then will check `PULUMI_CONFIG_PASSPHRASE_FILE` and then all through to
  asking interactively as the final option.
  [#5327](https://github.com/pulumi/pulumi/pull/5327)

- feat(autoapi): Add support for working with private Git repos. Either `SSHPrivateKeyPath`,
  `PersonalAccessToken` or `UserName` and `Password` can be pushed to the `auto.GitRepo` struct
  when interacting with a private repo
  [#5333](https://github.com/pulumi/pulumi/pull/5333)

- Revise the design for connecting an existing language runtime to a CLI invocation.
  Note that this is a protocol breaking change for the Automation API, so both the
  API and the CLI must be updated together.
  [#5317](https://github.com/pulumi/pulumi/pull/5317)

- Automation API - support streaming output for Up/Refresh/Destroy operations.
  [#5367](https://github.com/pulumi/pulumi/pull/5367)

- Automation API - add recovery APIs (cancel/export/import)
  [#5369](https://github.com/pulumi/pulumi/pull/5369)

## 2.10.0 (2020-09-10)

- feat(autoapi): add Upsert methods for stacks
  [#5316](https://github.com/pulumi/pulumi/pull/5316)

- Add IsSelectStack404Error and IsCreateStack409Error
  [#5314](https://github.com/pulumi/pulumi/pull/5314)

- Add internal scaffolding for cross-language components.
  [#5280](https://github.com/pulumi/pulumi/pull/5280)

- feat(autoapi): add workspace scoped envvars to LocalWorkspace and Stack
  [#5275](https://github.com/pulumi/pulumi/pull/5275)

- refactor(autoapi-gitrepo): use Workspace in SetupFn callback
  [#5279](https://github.com/pulumi/pulumi/pull/5279)

- Fix Go SDK plugin acquisition for programs with vendored dependencies
  [#5286](https://github.com/pulumi/pulumi/pull/5286)

- Python SDK: Add support for `Sequence[T]` for array types
  [#5282](https://github.com/pulumi/pulumi/pull/5282)

- feat(autoapi): Add support for non default secret providers in local workspaces
  [#5320](https://github.com/pulumi/pulumi/pull/5320)

- .NET SDK: Prevent a task completion race condition
  [#5324](https://github.com/pulumi/pulumi/pull/5324)

## 2.9.2 (2020-08-31)

- Alpha version of the Automation API for Go
  [#4977](https://github.com/pulumi/pulumi/pull/4977)

- Python SDK: Avoid raising an error when internal properties don't match the
  expected type.
  [#5251](https://github.com/pulumi/pulumi/pull/5251)

- Added `--suppress-permalink` option to suppress the permalink output
  (fixes [#4103](https://github.com/pulumi/pulumi/issues/4103))
  [#5191](https://github.com/pulumi/pulumi/pull/5191)

## 2.9.1 (2020-08-27)

- Python SDK: Avoid raising an error when an output has a type annotation of Any
  and the value is a list or dict.
  [#5238](https://github.com/pulumi/pulumi/pull/5238)

## 2.9.0 (2020-08-19)

- Fix support for CheckFailures in Python Dynamic Providers
  [#5138](https://github.com/pulumi/pulumi/pull/5138)

- Upgrade version of `gocloud.dev`. This ensures that 'AWSKMS' secrets
  providers can now be used with full ARNs rather than just Aliases
  [#5138](https://github.com/pulumi/pulumi/pull/5138)

- Ensure the 'history' command is a subcommand of 'stack'.
  This means that `pulumi history` has been deprecated in favour
  of `pulumi stack history`.
  [#5158](https://github.com/pulumi/pulumi/pull/5158)

- Add support for extracting jar files in archive resources
  [#5150](https://github.com/pulumi/pulumi/pull/5150)

- SDK changes to support Python input/output classes
  [#5033](https://github.com/pulumi/pulumi/pull/5033)

## 2.8.2 (2020-08-07)

- Add nuget badge to README [#5117](https://github.com/pulumi/pulumi/pull/5117)

- Support publishing and consuming Policy Packs using any runtime
  [#5102](https://github.com/pulumi/pulumi/pull/5102)

- Fix regression where any CLI integration for any stack with a default
  secrets provider would sort the config alphabetically and new stacks created
  would get created with an empty map `{}` in the config file
  [#5132](https://github.com/pulumi/pulumi/pull/5132)

## 2.8.1 (2020-08-05)

- Fix a bug where passphrase managers were not being
  recognised correctly when getting the configuration
  for the current stack.
  **Please Note:**
  This specific bug may have caused the stack config
  file to remove the password encryption salt.
  [#5110](https://github.com/pulumi/pulumi/pull/5110)

## 2.8.0 (2020-08-04)

- Add missing MapMap and ArrayArray types to Go SDK
  [#5092](https://github.com/pulumi/pulumi/pull/5092)

- Switch os/user package with luser drop in replacement
  [#5065](https://github.com/pulumi/pulumi/pull/5065)

- Update pip/setuptools/wheel in virtual environment before installing dependencies
  [#5042](https://github.com/pulumi/pulumi/pull/5042)

- Add ability to change a secrets provider for the current stack
  [#5031](https://github.com/pulumi/pulumi/pull/5031)

- Add ability to create a stack based on the config from an existing stack
  [#5062](https://github.com/pulumi/pulumi/pull/5062)

- Python: Improved error message when `virtualenv` doesn't exist
  [#5069](https://github.com/pulumi/pulumi/pull/5069)

- Enable pushing to Artifact Registry in actions
  [#5075](https://github.com/pulumi/pulumi/pull/5075)

## 2.7.1 (2020-07-22)

- Fix logic to parse pulumi venv on github action
  [5038](https://github.com/pulumi/pulumi/pull/5038)

## 2.7.0 (2020-07-22)

- Add pluginDownloadURL field to package definition
  [#4947](https://github.com/pulumi/pulumi/pull/4947)

- Add support for streamInvoke during update
  [#4990](https://github.com/pulumi/pulumi/pull/4990)

- Add ability to copy configuration values between stacks
  [#4971](https://github.com/pulumi/pulumi/pull/4971)

- Add logic to parse pulumi venv on github action
  [#4994](https://github.com/pulumi/pulumi/pull/4994)

- Better performance for stacks with many resources using the .NET SDK
  [#5015](https://github.com/pulumi/pulumi/pull/5015)

- Output PDB files and enable SourceLink integration for .NET assemblies
  [#4967](https://github.com/pulumi/pulumi/pull/4967)

## 2.6.1 (2020-07-09)

- Fix a panic in the display during CLI operations
  [#4987](https://github.com/pulumi/pulumi/pull/4987)

## 2.6.0 (2020-07-08)

- Go program gen: Improved handling for pulumi.Map types
  [#4914](https://github.com/pulumi/pulumi/pull/4914)

- Go SDK: Input type interfaces should declare pointer type impls where appropriate
  [#4911](https://github.com/pulumi/pulumi/pull/4911)

- Fixes issue where base64-encoded GOOGLE_CREDENTIALS causes problems with other commands
  [#4972](https://github.com/pulumi/pulumi/pull/4972)

## 2.5.0 (2020-06-25)

- Go program gen: prompt array conversion, unused range vars, id handling
  [#4884](https://github.com/pulumi/pulumi/pull/4884)

- Go program gen handling for prompt optional primitives
  [#4875](https://github.com/pulumi/pulumi/pull/4875)

- Go program gen All().Apply rewriter
  [#4858](https://github.com/pulumi/pulumi/pull/4858)

- Go program gen improvements (multiline strings, get/lookup disambiguation, invoke improvements)
  [#4850](https://github.com/pulumi/pulumi/pull/4850)

- Go program gen improvements (splat, all, index, traversal, range)
  [#4831](https://github.com/pulumi/pulumi/pull/4831)

- Go program gen improvements (resource range, readDir, fileArchive)
  [#4818](https://github.com/pulumi/pulumi/pull/4818)

- Set default config namespace for Get/Try/Require methods in Go SDK.
  [#4802](https://github.com/pulumi/pulumi/pull/4802)

- Handle invalid UTF-8 characters before RPC calls
  [#4816](https://github.com/pulumi/pulumi/pull/4816)

- Improve typing for Go SDK secret config values
  [#4800](https://github.com/pulumi/pulumi/pull/4800)

- Fix panic on `pulumi up` prompt after preview when filtering and hitting arrow keys.
  [#4808](https://github.com/pulumi/pulumi/pull/4808)

- Ensure GitHub Action authenticates to GCR when `$GOOGLE_CREDENTIALS` specified
  [#4812](https://github.com/pulumi/pulumi/pull/4812)

- Fix `pylint(no-member)` when accessing `resource.id`.
  [#4813](https://github.com/pulumi/pulumi/pull/4813)

- Fix GitHub Actions environment detection for PRs.
  [#4817](https://github.com/pulumi/pulumi/pull/4817)

- Adding language sdk specific docker containers.
  [#4837](https://github.com/pulumi/pulumi/pull/4837)

- Workaround bug in grcpio v1.30.0 by excluding this version from required dependencies.
  [#4883](https://github.com/pulumi/pulumi/pull/4883)

## 2.4.0 (2020-06-10)
- Turn program generation NYIs into diagnostic errors
  [#4794](https://github.com/pulumi/pulumi/pull/4794)

- Improve dev version detection logic
  [#4732](https://github.com/pulumi/pulumi/pull/4732)

- Export `CustomTimeouts` in the Python SDK
  [#4747](https://github.com/pulumi/pulumi/pull/4747)

- Add GitHub Actions CI detection
  [#4758](https://github.com/pulumi/pulumi/pull/4758)

- Allow users to specify base64 encoded strings as GOOGLE_CREDENTIALS
  [#4773](https://github.com/pulumi/pulumi/pull/4773)

- Install and use dependencies automatically for new Python projects.
  [#4775](https://github.com/pulumi/pulumi/pull/4775)

## 2.3.0 (2020-05-27)
- Add F# operators for InputUnion.
  [#4699](https://github.com/pulumi/pulumi/pull/4699)

- Add support for untagged outputs in Go SDK.
  [#4640](https://github.com/pulumi/pulumi/pull/4640)

- Update go-cloud to support all Azure regions
  [#4643](https://github.com/pulumi/pulumi/pull/4643)

- Fix a Regression in .NET unit testing.
  [#4656](https://github.com/pulumi/pulumi/pull/4656)

- Allow `pulumi.export` calls from Python unit tests.
  [#4670](https://github.com/pulumi/pulumi/pull/4670)

- Add support for publishing Python policy packs.
  [#4644](https://github.com/pulumi/pulumi/pull/4644)

- Improve download perf by fetching plugins from a CDN.
  [#4692](https://github.com/pulumi/pulumi/pull/4692)

## 2.2.1 (2020-05-13)
- Add new brew target to fix homebrew builds
  [#4633](https://github.com/pulumi/pulumi/pull/4633)

## 2.2.0 (2020-05-13)

- Fixed ResourceOptions issue with stack references in Python SDK
  [#4553](https://github.com/pulumi/pulumi/pull/4553)

- Add runTask to F# Deployment module
  [#3858](https://github.com/pulumi/pulumi/pull/3858)

- Add support for generating Fish completions
  [#4401](https://github.com/pulumi/pulumi/pull/4401)

- Support map-typed inputs in RegisterResource for Go SDK
  [#4522](https://github.com/pulumi/pulumi/pull/4522)

- Don't call IMocks.NewResourceAsync for the root stack resource
  [#4527](https://github.com/pulumi/pulumi/pull/4527)

- Add ResourceOutput type to Go SDK
  [#4575](https://github.com/pulumi/pulumi/pull/4575)

- Allow secrets to be decrypted when exporting a stack
  [#4046](https://github.com/pulumi/pulumi/pull/4046)

- Commands checking for a confirmation or requiring a `--yes` flag can now be
  skipped by setting `PULUMI_SKIP_CONFIRMATIONS` to `1` or `true`.
  [#4477](https://github.com/pulumi/pulumi/pull/4477)

## 2.1.1 (2020-05-11)

- Add retry support when writing to state buckets
  [#4494](https://github.com/pulumi/pulumi/pull/4494)

## 2.1.0 (2020-04-28)

- Fix infinite recursion bug for Go SDK
  [#4516](https://github.com/pulumi/pulumi/pull/4516)

- Order secretOutputNames when used in stack references
  [#4489](https://github.com/pulumi/pulumi/pull/4489)

- Add support for a `PULUMI_CONSOLE_DOMAIN` environment variable to override the
  behavior for how URLs to the Pulumi Console are generated.
  [#4410](https://github.com/pulumi/pulumi/pull/4410)

- Protect against panic when unprotecting non-existant resources
  [#4441](https://github.com/pulumi/pulumi/pull/4441)

- Add flag to `pulumi stack` to output only the stack name
  [#4450](https://github.com/pulumi/pulumi/pull/4450)

- Ensure Go accessor methods correctly support nested fields of optional outputs
  [#4456](https://github.com/pulumi/pulumi/pull/4456)

- Improve `ResourceOptions.merge` type in Python SDK
  [#4484](https://github.com/pulumi/pulumi/pull/4484)

- Ensure generated Python module names are keyword-safe.
  [#4473](https://github.com/pulumi/pulumi/pull/4473)

- Explicitly set XDG_CONFIG_HOME and XDG_CACHE_HOME env vars for helm in the
  pulumi docker image
  [#4474](https://github.com/pulumi/pulumi/pull/4474)

- Increase the MaxCallRecvMsgSize for all RPC calls.
  [#4455](https://github.com/pulumi/pulumi/pull/4455)

## 2.0.0 (2020-04-16)

- CLI behavior change.  Commands in non-interactive mode (i.e. when `pulumi` has its output piped to
  another process or running on CI) will not default to assuming that `--yes` was passed in.  `--yes` is now
  explicitly required to proceed in non-interactive scenarios. This affects:
  * `pulumi destroy`
  * `pulumi new`
  * `pulumi refresh`
  * `pulumi up`

- Fixed [crashes and hangs](https://github.com/pulumi/pulumi/issues/3528) introduced by usage of
  another library.

- @pulumi/pulumi now requires Node.js version >=10.10.0.

- All data-source invocations are now asynchronous (Promise-returning) by default.

- C# code generation switched to schema.

- .NET API: replace `IDeployment` interface with `DeploymentInstance` class.

- Fix Go SDK secret propagation for Resource inputs/outputs.
  [#4387](https://github.com/pulumi/pulumi/pull/4387)

- Fix Go codegen to emit config packages
  [#4388](https://github.com/pulumi/pulumi/pull/4388)

- Treat config values set with `--path` that start with '0' as strings rather than numbers.
  [#4393](https://github.com/pulumi/pulumi/pull/4393)

- Switch .NET projects to .NET Core 3.1
  [#4400](https://github.com/pulumi/pulumi/pull/4400)

- Avoid unexpected replace on resource with `import` applied on second update.
  [#4403](https://github.com/pulumi/pulumi/pull/4403)

## 1.14.1 (2020-04-13)
- Propagate `additionalSecretOutputs` opt to Read in NodeJS.
  [#4307](https://github.com/pulumi/pulumi/pull/4307)

- Fix handling of `nil` values in Outputs in Go.
  [#4268](https://github.com/pulumi/pulumi/pull/4268)

- Include usage hints for Input types in Go SDK
  [#4279](https://github.com/pulumi/pulumi/pull/4279)

- Fix secretness propagation in Python `apply`.
  [#4273](https://github.com/pulumi/pulumi/pull/4273)

- Fix the `call` mock in Python.
  [#4274](https://github.com/pulumi/pulumi/pull/4274)

- Fix handling of secret values in mock-based tests.
  [#4272](https://github.com/pulumi/pulumi/pull/4272)

- Automatic plugin acquisition for Go
  [#4297](https://github.com/pulumi/pulumi/pull/4297)

- Define merge behavior for resource options in Go SDK
  [#4316](https://github.com/pulumi/pulumi/pull/4316)

- Add overloads to Output.All in .NET
  [#4321](https://github.com/pulumi/pulumi/pull/4321)

- Make prebuilt executables opt-in only for the Go SDK
  [#4338](https://github.com/pulumi/pulumi/pull/4338)

- Support the `binary` option (prebuilt executables) for the .NET SDK
  [#4355](https://github.com/pulumi/pulumi/pull/4355)

- Add helper methods for stack outputs in the Go SDK
  [#4341](https://github.com/pulumi/pulumi/pull/4341)

- Add additional overloads to Deployment.RunAsync in .NET API.
  [#4286](https://github.com/pulumi/pulumi/pull/4286)

- Automate execution of `go mod download` for `pulumi new` Go templates
  [#4353](https://github.com/pulumi/pulumi/pull/4353)

- Fix `pulumi up -r -t $URN` not refreshing only the target
  [#4217](https://github.com/pulumi/pulumi/pull/4217)

- Fix logout with file backend when state is deleted
  [#4218](https://github.com/pulumi/pulumi/pull/4218)

- Fix specific flags for `pulumi stack` being global
  [#4294](https://github.com/pulumi/pulumi/pull/4294)

- Fix error when setting config without value in non-interactive mode
  [#4358](https://github.com/pulumi/pulumi/pull/4358)

- Propagate unknowns in Go SDK during marshal operations
  [#4369](https://github.com/pulumi/pulumi/pull/4369/files)

- Fix Go SDK stack reference helpers to handle nil values
  [#4370](https://github.com/pulumi/pulumi/pull/4370)

- Fix propagation of unknown status for secrets
  [#4377](https://github.com/pulumi/pulumi/pull/4377)

## 1.14.0 (2020-04-01)
- Fix error related to side-by-side versions of `@pulumi/pulumi`.
  [#4235](https://github.com/pulumi/pulumi/pull/4235)

- Allow users to specify an alternate backend URL when using the GitHub Actions container with the env var `PULUMI_BACKEND_URL`.
  [#4243](https://github.com/pulumi/pulumi/pull/4243)

## 1.13.1 (2020-03-27)
- Move to a multi-module repo to enable modules for the Go SDK
  [#4109](https://github.com/pulumi/pulumi/pull/4109)

- Report compile time errors for Go programs during plugin acquisition.
  [#4141](https://github.com/pulumi/pulumi/pull/4141)

- Add missing builtin `MapArray` to Go SDK.
  [#4144](https://github.com/pulumi/pulumi/pull/4144)

- Add aliases to Go SDK codegen pkg.
  [#4157](https://github.com/pulumi/pulumi/pull/4157)

- Discontinue testing on Node 8 (which has been end-of-life since January 2020), and start testing on Node 13.
  [#4156](https://github.com/pulumi/pulumi/pull/4156)

- Add support for enabling Policy Packs with configuration.
  [#3756](https://github.com/pulumi/pulumi/pull/4127)

- Remove obsolete .NET serialization attributes.
  [#4190](https://github.com/pulumi/pulumi/pull/4190)

- Add support for validating Policy Pack configuration.
  [#4179](https://github.com/pulumi/pulumi/pull/4186)

## 1.13.0 (2020-03-18)
- Add support for plugin acquisition for Go programs
  [#4060](https://github.com/pulumi/pulumi/pull/4060)

- Display resource type in PAC violation output
  [#4061](https://github.com/pulumi/pulumi/issues/4061)

- Update to Helm v3 in pulumi Docker image
  [#4090](https://github.com/pulumi/pulumi/pull/4090)

- Add ArrayMap builtin types to Go SDK
  [#4086](https://github.com/pulumi/pulumi/pull/4086)

- Improve documentation of URL formats for `pulumi login`
  [#4059](https://github.com/pulumi/pulumi/pull/4059)

- Add support for stack transformations in the .NET SDK.
  [#4008](https://github.com/pulumi/pulumi/pull/4008)

- Fix `pulumi stack ls` on Windows
  [#4094](https://github.com/pulumi/pulumi/pull/4094)

- Add support for running Python policy packs.
  [#4057](https://github.com/pulumi/pulumi/pull/4057)

## 1.12.1 (2020-03-11)
- Fix Kubernetes YAML parsing error in .NET.
  [#4023](https://github.com/pulumi/pulumi/pull/4023)

- Avoid projects beginning with `Pulumi` to stop cyclic imports
  [#4013](https://github.com/pulumi/pulumi/pull/4013)

- Ensure we can locate Go created application binaries on Windows
  [#4030](https://github.com/pulumi/pulumi/pull/4030)

- Ensure Python overlays work as part of our SDK generation
  [#4043](https://github.com/pulumi/pulumi/pull/4043)

- Fix terminal gets into a state where UP/DOWN don't work with prompts.
  [#4042](https://github.com/pulumi/pulumi/pull/4042)

- Ensure old provider is not used when configuration has changed
  [#4051](https://github.com/pulumi/pulumi/pull/4051)

- Support for unit testing and mocking in the .NET SDK.
  [#3696](https://github.com/pulumi/pulumi/pull/3696)

## 1.12.0 (2020-03-04)
- Avoid Configuring providers which are not used during preview.
  [#4004](https://github.com/pulumi/pulumi/pull/4004)

- Fix missing module import on Windows platform.
  [#3983](https://github.com/pulumi/pulumi/pull/3983)

- Add support for mocking the resource monitor to the NodeJS and Python SDKs.
  [#3738](https://github.com/pulumi/pulumi/pull/3738)

- Reinstate caching of TypeScript compilation.
  [#4007](https://github.com/pulumi/pulumi/pull/4007)

- Remove the need to set PULUMI_EXPERIMENTAL to use the policy and watch commands.
  [#4001](https://github.com/pulumi/pulumi/pull/4001)

- Fix type annotations for `Output.all` and `Output.concat` in Python SDK.
  [#4016](https://github.com/pulumi/pulumi/pull/4016)

- Add support for configuring policies.
  [#4015](https://github.com/pulumi/pulumi/pull/4015)

## 1.11.1 (2020-02-26)
- Fix a regression for CustomTimeouts in Python SDK.
  [#3964](https://github.com/pulumi/pulumi/pull/3964)

- Avoid panic when displaying failed stack policies.
  [#3960](https://github.com/pulumi/pulumi/pull/3960)

- Add support for secrets in the Go SDK.
  [3938](https://github.com/pulumi/pulumi/pull/3938)

- Add support for transformations in the Go SDK.
  [3978](https://github.com/pulumi/pulumi/pull/3938)

## 1.11.0 (2020-02-19)
- Allow oversize protocol buffers for Python SDK.
  [#3895](https://github.com/pulumi/pulumi/pull/3895)

- Avoid duplicated messages in preview/update progress display.
  [#3890](https://github.com/pulumi/pulumi/pull/3890)

- Improve CPU utilization in the Python SDK when waiting for resource operations.
  [#3892](https://github.com/pulumi/pulumi/pull/3892)

- Expose resource options, parent, dependencies, and provider config to policies.
  [#3862](https://github.com/pulumi/pulumi/pull/3862)

- Move .NET SDK attributes to the root namespace.
  [#3902](https://github.com/pulumi/pulumi/pull/3902)

- Support exporting older stack versions.
  [#3906](https://github.com/pulumi/pulumi/pull/3906)

- Disable interactive progress display when no terminal size is available.
  [#3936](https://github.com/pulumi/pulumi/pull/3936)

- Mark `ResourceOptions` class as abstract in the .NET SDK. Require the use of derived classes.
  [#3943](https://github.com/pulumi/pulumi/pull/3943)

## 1.10.1 (2020-02-06)
- Support stack references in the Go SDK.
  [#3829](https://github.com/pulumi/pulumi/pull/3829)

- Fix the Windows release process.
  [#3875](https://github.com/pulumi/pulumi/pull/3875)

## 1.10.0 (2020-02-05)
- Avoid writing checkpoints to backend storage in common case where no changes are being made.
  [#3860](https://github.com/pulumi/pulumi/pull/3860)

- Add information about an in-flight operation to the stack command output, if applicable.
  [#3822](https://github.com/pulumi/pulumi/pull/3822)

- Update `SummaryEvent` to include the actual name and local file path for locally-executed policy packs.

- Add support for aliases in the Go SDK
  [3853](https://github.com/pulumi/pulumi/pull/3853)

- Fix Python Dynamic Providers on Windows.
  [#3855](https://github.com/pulumi/pulumi/pull/3855)

## 1.9.1 (2020-01-27)
- Fix a stack reference regression in the Python SDK.
  [#3798](https://github.com/pulumi/pulumi/pull/3798)

- Fix a buggy assertion in the Go SDK.
  [#3794](https://github.com/pulumi/pulumi/pull/3794)

- Add `--latest` flag to `pulumi policy enable`.

- Breaking change for Policy which removes requirement for version when running `pulumi policy disable`. Add `--version` flag if user wants to specify version of Policy Pack to disable.

- Fix rendering of Policy Packs to ensure they are always displayed.

- Primitive input types in the Go SDK (e.g. Int, String, etc.) now implement the corresponding Ptr type e.g. IntPtr,
  StringPtr, etc.). This is consistent with the output of the Go code generator and is much more ergonomic for
  optional inputs than manually converting to pointer types.
  [#3806](https://github.com/pulumi/pulumi/pull/3806)

- Add ability to specify all versions when removing a Policy Pack.

- Breaking change to Policy command: Change enable command to use `pulumi policy enable <org-name>/<policy-pack-name> latest` instead of a `--latest` flag.

## 1.9.0 (2020-01-22)
- Publish python types for PEP 561
  [#3704](https://github.com/pulumi/pulumi/pull/3704)

- Lock dep ts-node to v8.5.4
  [#3733](https://github.com/pulumi/pulumi/pull/3733)

- Improvements to `pulumi policy` functionality. Add ability to remove & disable Policy Packs.

- Breaking change for Policy which is in Public Preview: Change `pulumi policy apply` to `pulumi policy enable`, and allow users to specify the Policy Group.

- Add Permalink to output when publishing a Policy Pack.

- Add `pulumi policy ls` and `pulumi policy group ls` to list Policy related resources.

- Add `BuildNumber` to CI vars and backend metadata property bag for CI systems that have separate ID and a user-friendly number. [#3766](https://github.com/pulumi/pulumi/pull/3766)

- Breaking changes for the Go SDK. Complete details are in [#3506](https://github.com/pulumi/pulumi/pull/3506).

## 1.8.1 (2019-12-20)

- Fix a panic in `pulumi stack select`.
  [#3687](https://github.com/pulumi/pulumi/pull/3687)

## 1.8.0 (2019-12-19)

- Update version of TypeScript used by Pulumi to `3.7.3`.
  [#3627](https://github.com/pulumi/pulumi/pull/3627)

- Add support for GOOGLE_CREDENTIALS when using Google Cloud Storage backend.
  [#2906](https://github.com/pulumi/pulumi/pull/2906)

  ```sh
   export GOOGLE_CREDENTIALS="$(cat ~/service-account-credentials.json)"
   pulumi login gs://my-bucket
  ```


- Support for using `Config`, `getProject()`, `getStack()`, and `isDryRun()` from Policy Packs.
  [#3612](https://github.com/pulumi/pulumi/pull/3612)

- Top-level Stack component in the .NET SDK.
  [#3618](https://github.com/pulumi/pulumi/pull/3618)

- Add the .NET Core 3.0 runtime to the `pulumi/pulumi` container.
  [#3616](https://github.com/pulumi/pulumi/pull/3616)

- Add `pulumi preview` support for `--refresh`, `--target`, `--replace`, `--target-replace` and
  `--target-dependents` to align with `pulumi up`.
  [#3675](https://github.com/pulumi/pulumi/pull/3675)

- `ComponentResource`s now have built-in support for asynchronously constructing their children.
  [#3676](https://github.com/pulumi/pulumi/pull/3676)

- `Output.apply` (for the JS, Python and .Net sdks) has updated semantics, and will lift dependencies from inner Outputs to the returned Output.
  [#3663](https://github.com/pulumi/pulumi/pull/3663)

- Fix bug in determining PRNumber and BuildURL for an Azure Pipelines CI environment.
  [#3677](https://github.com/pulumi/pulumi/pull/3677)

- Improvements to `pulumi policy` functionality. Add ability to remove & disable Policy Packs.

- Breaking change for Policy which is in Public Preview: Change `pulumi policy apply` to `pulumi policy enable`, and allow users to specify the Policy Group.

## 1.7.1 (2019-12-13)

- Fix [SxS issue](https://github.com/pulumi/pulumi/issues/3652) introduced in 1.7.0 when assigning
  `Output`s across different versions of the `@pulumi/pulumi` SDK.
  [#3658](https://github.com/pulumi/pulumi/pull/3658)

## 1.7.0 (2019-12-11)

- A Pulumi JavaScript/TypeScript program can now consist of a single exported top level function. This
  allows for an easy approach to create a Pulumi program that needs to perform `async`/`await`
  operations at the top-level.
  [#3321](https://github.com/pulumi/pulumi/pull/3321)

  ```ts
  // JavaScript
  module.exports = async () => {
  }

  //TypeScript
  export = async () => {
  }
  ```

## 1.6.1 (2019-11-26)

- Support passing a parent and providers for `ReadResource`, `RegisterResource`, and `Invoke` in the go SDK. [#3563](https://github.com/pulumi/pulumi/pull/3563)

- Fix go SDK ReadResource.
  [#3581](https://github.com/pulumi/pulumi/pull/3581)

- Fix go SDK DeleteBeforeReplace.
  [#3572](https://github.com/pulumi/pulumi/pull/3572)

- Support for setting the `PULUMI_PREFER_YARN` environment variable to opt-in to using `yarn` instead of `npm` for
  installing Node.js dependencies.
  [#3556](https://github.com/pulumi/pulumi/pull/3556)

- Fix regression that prevented relative paths passed to `--policy-pack` from working.
  [#3565](https://github.com/pulumi/pulumi/issues/3564)

## 1.6.0 (2019-11-20)

- Support for config.GetObject and related variants for Golang.
  [#3526](https://github.com/pulumi/pulumi/pull/3526)

- Add support for IgnoreChanges in the go SDK.
  [#3514](https://github.com/pulumi/pulumi/pull/3514)

- Support for a `go run` style workflow. Building or installing a pulumi program written in go is
  now optional.
  [#3503](https://github.com/pulumi/pulumi/pull/3503)

- Re-apply "propagate resource inputs to resource state during preview, including first-class unknown values." The new
  set of changes have additional fixes to ensure backwards compatibility with earlier code. This allows the preview to
  better estimate the state of a resource after an update, including property values that were populated using defaults
  calculated by the provider.
  [#3327](https://github.com/pulumi/pulumi/pull/3327)

- Validate StackName when passing a non-default secrets provider to `pulumi stack init`

- Add support for go1.13.x

- `pulumi update --target` and `pulumi destroy --target` will both error if they determine a
  dependent resource needs to be updated, destroyed, or created that was not was specified in the
  `--target` list.  To proceed with an `update/destroy` after this error, either specify all the
  reported resources as `--target`s, or pass the `--target-dependents` flag to allow necessary
  changes to unspecified dependent targets.

- Support for node 13.x, building with gcc 8 and newer.
  [#3512] (https://github.com/pulumi/pulumi/pull/3512)

- Codepaths which could result in a hang will print a message to the console indicating the problem, along with a link
  to documentation on how to restructure code to best address it.

### Compatibility

- `StackReference.getOutputSync` and `requireOutputSync` are deprecated as they may cause hangs on
  some combinations of Node and certain OS platforms. `StackReference.getOutput` and `requireOutput`
  should be used instead.

## 1.5.2 (2019-11-13)

- `pulumi policy publish` now determines the Policy Pack name from the Policy Pack, and the
  the `org-name` CLI argument is now optional. If not specified; the current user account is
  used.
  [#3459](https://github.com/pulumi/pulumi/pull/3459)

- Refactor the Output API in the Go SDK.
  [#3496](https://github.com/pulumi/pulumi/pull/3496)

## 1.5.1 (2019-11-06)

- Include the .NET language provider in the Windows SDK.

## 1.5.0 (2019-11-06)

- Gracefully handle errors when resources use duplicate aliases.

- Use the update token for renew_lease calls and update the API version to 5.
  [#3348](https://github.com/pulumi/pulumi/pull/3348)

- Improve startup time performance by 0.5-1s by checking for a newer CLI release in parallel.
  [#3441](https://github.com/pulumi/pulumi/pull/3441)

- Add an experimental `pulumi watch` command.
  [#3391](https://github.com/pulumi/pulumi/pull/3391)

## 1.4.1 (2019-11-01)

- Adds a **preview** of .NET support for Pulumi. This code is an preview state and is subject
  to change at any point.

- Fix another colorizer issue that could cause garbled output for messages that did not end in colorization tags.
  [#3417](https://github.com/pulumi/pulumi/pull/3417)

- Verify deployment integrity during import and issue an error if verification fails. The state file can still be
  imported by passing the `--force` flag.
  [#3422](https://github.com/pulumi/pulumi/pull/3422)

- Omit unknowns in resources in stack outputs during preview.
  [#3427](https://github.com/pulumi/pulumi/pull/3427)

- `pulumi update` can now be instructed that a set of resources should be replaced by adding a
  `--replace urn` argument.  Multiple resources can be specified using `--replace urn1 --replace urn2`. In order to
  replace exactly one resource and leave other resources unchanged, invoke `pulumi update --replace urn --target urn`,
  or `pulumi update --target-replace urn` for short.
  [#3418](https://github.com/pulumi/pulumi/pull/3418)

- `pulumi stack` now renders the stack as a tree view.
  [#3430](https://github.com/pulumi/pulumi/pull/3430)

- Support for lists and maps in config.
  [#3342](https://github.com/pulumi/pulumi/pull/3342)

- `ResourceProvider#StreamInvoke` implemented, will be the basis for streaming
  APIs in `pulumi query`.
  [#3424](https://github.com/pulumi/pulumi/pull/3424)

## 1.4.0 (2019-10-24)

- `FileAsset` in the Python SDK now accepts anything implementing `os.PathLike` in addition to `str`.
  [#3368](https://github.com/pulumi/pulumi/pull/3368)

- Fix colorization on Windows 10, and fix a colorizer bug that could cause garbled output for resources with long
  status messages.
  [#3385](https://github.com/pulumi/pulumi/pull/3385)

## 1.3.4 (2019-10-18)

- Remove unintentional console outupt introduced in 1.3.3.

## 1.3.3 (2019-10-17)

- Fix an issue with first-class providers introduced in 1.3.2.

## 1.3.2 (2019-10-16)

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

- Add `helm` to `pulumi/pulumi` Dockerhub container
  [#3294](https://github.com/pulumi/pulumi/pull/3294)

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
