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

- [sdk/nodejs] Add support for multiple V8 VM contexts in closure serialization.
  [#6648](https://github.com/pulumi/pulumi/pull/6648)

- [cli] Add option to print absolute rather than relative dates in stack history
  [#6742](https://github.com/pulumi/pulumi/pull/6742)

- [sdk/dotnet] Thread-safe concurrency-friendly global state
  [#6139](https://github.com/pulumi/pulumi/pull/6139)

- [cli] Enable absolute and relative parent paths for pulumi main
  [#6734](https://github.com/pulumi/pulumi/pull/6734)

- [automation/python] Update pulumi python docker image to python 3.9
  [#6706](https://github.com/pulumi/pulumi/pull/6706)

- [sdk/nodejs] Add program side caching for dynamic provider serialization behind env var
  [#6673](https://github.com/pulumi/pulumi/pull/6673)

- [automation/dotnet] Allow null environment variables
  [#6687](https://github.com/pulumi/pulumi/pull/6687)

- [automation/dotnet] Expose WorkspaceStack.GetOutputsAsync
  [#6699](https://github.com/pulumi/pulumi/pull/6699)

- [automation/dotnet] Implement (Import,Export)StackAsync methods on LocalWorkspace and WorkspaceStack and expose StackDeployment helper class.
  [#6728](https://github.com/pulumi/pulumi/pull/6728)

- [sdk/nodejs] Allow prompt values in `construct` for multi-lang components.
  [#6522](https://github.com/pulumi/pulumi/pull/6522)

### Bug Fixes

- [cli] Handle non-existent creds file in `pulumi logout --all`
  [#6741](https://github.com/pulumi/pulumi/pull/6741)

- [automation] Set default value for 'main' for inline programs to support relative paths, assets, and closure serialization.
  [#6743](https://github.com/pulumi/pulumi/pull/6743)

- [sdk/nodejs] Explicitly create event log file for NodeJS Automation API.
  [#6730](https://github.com/pulumi/pulumi/pull/6730)

- [sdk/nodejs] Fix error handling for failed logging statements
  [#6714](https://github.com/pulumi/pulumi/pull/6714)

- [automation/dotnet] Environment variable value type is now nullable.
  [#6520](https://github.com/pulumi/pulumi/pull/6520)

- [sdk/nodejs] Fix `Construct` to wait for child resources of a multi-lang components to be created.
  [#6452](https://github.com/pulumi/pulumi/pull/6452

- [sdk/python] Fix serialization bug if output contains 'items' property.
  [#6701](https://github.com/pulumi/pulumi/pull/6701)

- [sdk/go] Use ioutil.ReadFile to avoid forcing 1.16 upgrade.
  [#6703](https://github.com/pulumi/pulumi/pull/6703)

- [automation/dotnet] Fix GetConfigValueAsync failing to deserialize
  [#6698](https://github.com/pulumi/pulumi/pull/6698)
