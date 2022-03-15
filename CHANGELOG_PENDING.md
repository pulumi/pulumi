### Improvements

- [area/cli] - Implement `pulumi stack unselect` [#9179](https://github.com/pulumi/pulumi/pull/9179)
- [language/dotnet] - Updated Pulumi dotnet packages to use grpc-dotnet instead of grpc.
   [#9149](https://github.com/pulumi/pulumi/pull/9149)

- [cli/config] - Rename the `config` property in `Pulumi.yaml` to `stackConfigDir`. The `config` key will continue to be supported.
  [#9145](https://github.com/pulumi/pulumi/pull/9145)

- [cli] - Speed up `pulumi stack --show-name` by skipping unneeded snapshot loading.
  [#9199](https://github.com/pulumi/pulumi/pull/9199)

### Bug Fixes

  [sdk/nodejs] - Fix uncaught error "ENOENT: no such file or directory" when an error occurs during the stack up
  [#9065](https://github.com/pulumi/pulumi/issues/9065)
