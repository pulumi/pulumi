### Improvements

- [sdk/go] - Improve error messages for (un)marshalling properties.
  [#7936](https://github.com/pulumi/pulumi/pull/7936)

- [sdk/go] - Initial support for (un)marshalling output values.
  [#7861](https://github.com/pulumi/pulumi/pull/7861)

- [sdk/go] - Add `RegisterInputType` and register built-in types.
  [#7928](https://github.com/pulumi/pulumi/pull/7928)

- [codegen] - Packages include `Package.Version` when provided.
  [#7938](https://github.com/pulumi/pulumi/pull/7938)

- [schema] The syntactical well-formedness of a package schema is now described
  and checked by a JSON schema metaschema.
  [#7952](https://github.com/pulumi/pulumi/pull/7952)

### Bug Fixes

- [codegen/schema] - Correct validation for Package
  [#7896](https://github.com/pulumi/pulumi/pull/7896)

- [cli] Use json.Unmarshal instead of custom parser
  [#7954](https://github.com/pulumi/pulumi/pull/7954)

- [sdk/{go,dotnet}] - Thread replaceOnChanges through Go and .NET
  [#7967](https://github.com/pulumi/pulumi/pull/7967)

- [codegen/nodejs] - Correctly handle hyphenated imports
  [#7993](https://github.com/pulumi/pulumi/pull/7993)
