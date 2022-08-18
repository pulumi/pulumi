### Improvements

- [provider/python]: Improved exception display. The traceback is now shorter and it always starts with user code.  
  [#10336](https://github.com/pulumi/pulumi/pull/10336)

- [sdk/python] Update PyYAML to 6.0

- [cli/watch] `pulumi watch` now uses relies on a program built on [`watchexec`](https://github.com/watchexec/watchexec)
  to implement recursive file watching, improving performance and cross-platform compatibility.
  This `pulumi-watch` program is now included in releases.
  [#10213](https://github.com/pulumi/pulumi/issues/10213)


- [testing] Implement matrix testing [#10231](https://github.com/pulumi/pulumi/pull/10231)

- [codegen] Minor modifications to nodejs, dotnet, go, typescript to allow generating test packages not intended for release [#10231](https://github.com/pulumi/pulumi/pull/10231)

- [testing] Add PulumiBin and AuxiliaryStack options to integration testing [#10231](https://github.com/pulumi/pulumi/pull/10231)
### Bug Fixes

- [engine/backends]: Fix bug where File state backend failed to apply validation to stack names, resulting in a panic.
  [#10417](https://github.com/pulumi/pulumi/pull/10417)
