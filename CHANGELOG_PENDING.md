### Improvements

- [cli/import] - The import command no longer errors if resource properties do not validate. Instead the
  engine warns about property issues returned by the provider but then continues with the import and codegen
  as best it can. This should result in more resources being imported to the pulumi state and being able to
  generate some code, at the cost that the generated code may not work as is in an update. Users will have to
  edit the code to succesfully run.
  [#8922](https://github.com/pulumi/pulumi/pull/8922)

### Bug Fixes

- [sdk/go] - Normalize merge behavior for `ResourceOptions`, inline
  with other SDKs. See https://github.com/pulumi/pulumi/issues/8796 for more
  details.
  [#8882](https://github.com/pulumi/pulumi/pull/8882)

- [sdk/nodejs] - Fix nodejs function serialization module path to comply with package.json exports if exports is specified.
  [#8893](https://github.com/pulumi/pulumi/pull/8893)
