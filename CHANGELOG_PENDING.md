### Breaking Changes

### Improvements

- [auto/dotnet] - Provide PulumiFn implementation that allows runtime stack type
  [#6910](https://github.com/pulumi/pulumi/pull/6910)

- [auto/go] - Provide GetPermalink for all results
  [#6875](https://github.com/pulumi/pulumi/pull/6875)

### Bug Fixes

- [sdk/python] Fix relative `runtime:options:virtualenv` path resolution to ignore `main` project attribute
  [#6966](https://github.com/pulumi/pulumi/pull/6966)

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

- Config: Avoid emitting integers in objects using exponential notation.
  [#7005](https://github.com/pulumi/pulumi/pull/7005)

- Build: Add vs code dev container
  [#7052](https://github.com/pulumi/pulumi/pull/7052)

- Ensure that all outstanding asynchronous work is awaited before returning from a Go
  Pulumi program. Note that this may require changes to programs that use the
  `pulumi.NewOutput` API.
  [#6983](https://github.com/pulumi/pulumi/pull/6983)