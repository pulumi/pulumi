### Improvements

- [auto/*] Add `--policy-pack` and `--policy-pack-configs` options to automation API.
  [#9872](https://github.com/pulumi/pulumi/pull/9872)

- [cli] The engine now produces a warning when the 'ignoreChanges' option is applied to a Component resource.
  [#9863](https://github.com/pulumi/pulumi/pull/9863)

- [sdk/python] Changed `Output[T].__str__()` to return an informative message rather than "<pulumi.output.Output object at 0x012345ABCDEF>".
  [#9848](https://github.com/pulumi/pulumi/pull/9848)

- [cli] The engine will now default resource parent to the root stack if it exists.
  [#9481](https://github.com/pulumi/pulumi/pull/9481)

- [engine] Reduce memory usage in convert and yaml programs by caching of package schemas.
  [#9684](https://github.com/pulumi/pulumi/issues/9684)

- [sdk/go] Added `NewUniqueName` for providers to use for generating autonames.
  [#9852](https://github.com/pulumi/pulumi/pull/9852)

- [engine] The engine now understands alias objects which it can resolve to URNs, requiring less logic in SDKs.
  [#9731](https://github.com/pulumi/pulumi/pull/9731)

- [sdk/dotnet] The dotnet SDK will now send alias objects rather than URNs to the engine.
  [#9731](https://github.com/pulumi/pulumi/pull/9731)

- [engine] The engine will default to delete before create for replacements when it has no information from the provider that create before delete is safe.
  [#9909](https://github.com/pulumi/pulumi/pull/9909)

### Bug Fixes

- [sdk/go] Handle nils in mapper encoding.
  [#9810](https://github.com/pulumi/pulumi/pull/9810)

- [engine] Explicit providers use the same plugin as default providers unless otherwise requested.
  [#9708](https://github.com/pulumi/pulumi/pull/9708)

- [sdk/go] Correctly parse nested git projects in GitLab.
  [#9354](https://github.com/pulumi/pulumi/issues/9354)

- [sdk/go] Mark StackReference keys that don't exist as unknown. Error when converting unknown keys to strings.
  [#9855](https://github.com/pulumi/pulumi/pull/9855)

- [sdk/go] Precisely mark values obtained via stack reference `Get...Output(key)` methods as secret or not.
  [#9842](https://github.com/pulumi/pulumi/pull/9842)
