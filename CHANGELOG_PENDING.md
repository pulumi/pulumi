### Breaking Changes



### Improvements

- [auto/dotnet] - Provide PulumiFn implementation that allows runtime stack type
  [#6910](https://github.com/pulumi/pulumi/pull/6910)

- [auto/go] - Provide GetPermalink for all results
  [#6875](https://github.com/pulumi/pulumi/pull/6875)

### Bug Fixes

- [auto/dotnet] - Disable Language Server Host logging and checking appsettings.json config
  [#7023](https://github.com/pulumi/pulumi/pull/7023)

- [auto/python] - Export missing `ProjectBackend` type
  [#6984](https://github.com/pulumi/pulumi/pull/6984)

- [sdk/nodejs] - Fix noisy errors.
  [#6995](https://github.com/pulumi/pulumi/pull/6995)

- Config: Avoid emitting integers in objects using exponential notation.
  [#7005](https://github.com/pulumi/pulumi/pull/7005)

- [codegen/python] - Fix issue with lazy_import affecting pulumi-eks
  [#7024](https://github.com/pulumi/pulumi/pull/7024)

- Ensure that all outstanding asynchronous work is awaited before returning from a .NET
  Pulumi program.
  [#6993](https://github.com/pulumi/pulumi/pull/6993)
