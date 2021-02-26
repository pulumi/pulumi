### Features


### Enhancements

- [#6410](https://github.com/pulumi/pulumi/pull/6410) Add `diff` option to Automation API's `preview` and `up`

### Bug Fixes

- [automation/python,nodejs,dotnet] - BREAKING - Remove `summary` property from `PreviewResult`.
  The `summary` property on `PreviewResult` returns a result that is always incorrect and is being removed.
  [#6405](https://github.com/pulumi/pulumi/pull/6405)
  
- [automation/python] - Fix Windows error caused by use of NamedTemporaryFile in automation api.
  [#6421](https://github.com/pulumi/pulumi/pull/6421)
