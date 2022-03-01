### Improvements

- [cli/import] - The import command no longer errors if resource properties do not validate. Instead the
  engine warns about property issues returned by the provider but then continues with the import and codegen
  as best it can. This should result in more resources being imported to the pulumi state and being able to
  generate some code, at the cost that the generated code may not work as is in an update. Users will have to
  edit the code to succesfully run.
  [#8922](https://github.com/pulumi/pulumi/pull/8922)

### Bug Fixes

- [sdk/nodejs] - Fix Node `fs.rmdir` DeprecationWarning for Node JS 15.X+
  [#9044](https://github.com/pulumi/pulumi/pull/9044)

- [engine] - Fix deny default provider handling for Invokes and Reads.
  [#9067](https://github.com/pulumi/pulumi/pull/9067)

- [codegen/go] - Fix secret codegen for input properties
  [#9052](https://github.com/pulumi/pulumi/pull/9052)

- [sdk/nodejs] - `PULUMI_NODEJS_TSCONFIG_PATH` is now explicitly passed to tsnode for the tsconfig file.
  [#9062](https://github.com/pulumi/pulumi/pull/9062)
