### Improvements

- [codegen/dotnet] - Add C# extension `rootNamespace`, allowing the user to
  replace `Pulumi` as the default C# global namespace in generated programs.
  The `Company` and `Author` fields of the .csproj file are not driven by
  `schema.publisher`.
  [#8735](https://github.com/pulumi/pulumi/pull/8735)

### Bug Fixes

