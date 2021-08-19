### Improvements

- [sdk/python] - Add support for custom naming of dynamic provider resource.
  [#7633](https://github.com/pulumi/pulumi/pull/7633)

### Bug Fixes

- [codegen/go] - Fix nested collection type generation.
  [#7779](https://github.com/pulumi/pulumi/pull/7779)

- [sdk/dotnet] - Fix an exception when passing an unknown `Output` to
  the `DependsOn` resource option.
  [#7762](https://github.com/pulumi/pulumi/pull/7762)

- [engine] Include transitive children in dependency list for deletes.
  [#7788](https://github.com/pulumi/pulumi/pull/7788)
