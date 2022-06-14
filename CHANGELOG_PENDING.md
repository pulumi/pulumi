### Improvements

- [sdk/python] Changed `Output[T].__str__()` to return an informative message rather than "<pulumi.output.Output object at 0x012345ABCDEF>".
  [#9848](https://github.com/pulumi/pulumi/pull/9848)

- [cli] The engine will now default resource parent to the root stack if it exists.
  [#9481](https://github.com/pulumi/pulumi/pull/9481)

- [engine] Reduce memory usage in convert and yaml programs by caching of package schemas.
  [#9684](https://github.com/pulumi/pulumi/issues/9684)

### Bug Fixes

- [engine] Explicit providers use the same plugin as default providers unless otherwise requested.
  [#9708](https://github.com/pulumi/pulumi/pull/9708)

- [sdk/go] Correctly parse nested git projects in GitLab.
  [#9354](https://github.com/pulumi/pulumi/issues/9354)

- [sdk/go] Mark StackReference keys that don't exist as unknown. Error when converting unknown keys to strings.
  [#9855](https://github.com/pulumi/pulumi/pull/9855)

- [sdk/go] Precisely mark values obtained via stack reference `Get...Output(key)` methods as secret or not.
  [#9842](https://github.com/pulumi/pulumi/pull/9842)
