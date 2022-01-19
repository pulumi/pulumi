### Improvements

- [sdk/dotnet] - Changed `Output<T>.ToString()` to return an informative message rather than just "Output`1[X]"
  [#8767](https://github.com/pulumi/pulumi/pull/8767)

### Bug Fixes

- [codegen/nodejs] - Generate an install script that runs `pulumi plugin install` with 
  the `--server` flag when necessary.
  [#8730](https://github.com/pulumi/pulumi/pull/8730)
