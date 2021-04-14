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
