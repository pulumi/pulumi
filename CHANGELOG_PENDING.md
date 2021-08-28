### Improvements

- [auto/dotnet] - Add support for `--exact` and `--server` with `pulumi plugin install` via Automation API. BREAKING NOTE: If you are subclassing `Workspace` your `InstallPluginAsync` implementation will need to be updated to reflect the new `PluginInstallOptions` parameter.
  [#7762](https://github.com/pulumi/pulumi/pull/7796)

- [codegen/go] - Add helper function forms `$fnOutput` that accept
  `Input`s, return an `Output`, and wrap the underlying `$fn` call.
  This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for Go, making it
  easier to compose functions/datasources with Pulumi resources.
  [#7784](https://github.com/pulumi/pulumi/pull/7784)

- [sdk/python] - Speed up `pulumi up` on Python projects by optimizing
  `pip` invocations
  [#7819](https://github.com/pulumi/pulumi/pull/7819)

- [sdk/dotnet] - Support for calling methods.
  [#7582](https://github.com/pulumi/pulumi/pull/7582)
  
- [build] - make lint returns an accurate status code 
  [#7844](https://github.com/pulumi/pulumi/pull/7844)

### Bug Fixes

- [cli] - Avoid `missing go.sum entry for module` for new Go projects.
  [#7808](https://github.com/pulumi/pulumi/pull/7808)

- [codegen/schema] - Allow hyphen in schema path reference.
  [#7824](https://github.com/pulumi/pulumi/pull/7824)
