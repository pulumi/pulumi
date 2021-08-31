### Improvements

- [auto/dotnet] - Add support for `--exact` and `--server` with `pulumi plugin install` via Automation API. BREAKING NOTE: If you are subclassing `Workspace` your `InstallPluginAsync` implementation will need to be updated to reflect the new `PluginInstallOptions` parameter.
  [#7762](https://github.com/pulumi/pulumi/pull/7796)

- [codegen/go] - Add helper function forms `$fnOutput` that accept
  `Input`s, return an `Output`, and wrap the underlying `$fn` call.
  This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for Go, making it
  easier to compose functions/datasources with Pulumi resources.
  [#7784](https://github.com/pulumi/pulumi/pull/7784)

- [codegen] - Add `replaceOnChange` to schema.
  [#7874](https://github.com/pulumi/pulumi/pull/7874)

### Bug Fixes

- [cli] - Avoid `missing go.sum entry for module` for new Go projects.
  [#7808](https://github.com/pulumi/pulumi/pull/7808)

- [codegen/schema] - Allow hyphen in schema path reference.
  [#7824](https://github.com/pulumi/pulumi/pull/7824)
