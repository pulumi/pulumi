### Improvements

- [cli/backend] - `pulumi cancel` is now supported for the file state backend.
  [#9100](https://github.com/pulumi/pulumi/pull/9100)

### Bug Fixes

- [sdk/nodejs] - Fix Node `fs.rmdir` DeprecationWarning for Node JS 15.X+
  [#9044](https://github.com/pulumi/pulumi/pull/9044)

- [engine] - Fix deny default provider handling for Invokes and Reads.
  [#9067](https://github.com/pulumi/pulumi/pull/9067)

- [codegen/go] - Fix secret codegen for input properties
  [#9052](https://github.com/pulumi/pulumi/pull/9052)

- [sdk/nodejs] - `PULUMI_NODEJS_TSCONFIG_PATH` is now explicitly passed to tsnode for the tsconfig file.
  [#9062](https://github.com/pulumi/pulumi/pull/9062)
