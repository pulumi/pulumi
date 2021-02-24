### Breaking changes

Note: Breaking changes should be expected outside of major version changes while Automation API is in preview.

- [automation/python,nodejs,dotnet] - Remove `summary` property from `PreviewResult`.
  The `summary` property on `PreviewResult` returns a result that is always incorrect and is being removed.
  [#6405](https://github.com/pulumi/pulumi/pull/6405)
  
- [automation/go] - Add a `ProgressStreams` option to Stack.Preview().
  This is a breaking change as it changes the shape of `PreviewResult`.
  [#6308](https://github.com/pulumi/pulumi/pull/6308)

### Features


### Enhancements

- Add `diff` option to Automation API's `preview` and `up`.
  [#6410](https://github.com/pulumi/pulumi/pull/6410)

### Bug Fixes
