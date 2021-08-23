### Improvements

- [auto/dotnet] - Add support for `--exact` and `--server` with `pulumi plugin install` via Automation API. BREAKING NOTE: If you are subclassing `Workspace` your `InstallPluginAsync` implementation will need to be updated to reflect the new `PluginInstallOptions` parameter.
  [#7762](https://github.com/pulumi/pulumi/pull/7796)

### Bug Fixes


