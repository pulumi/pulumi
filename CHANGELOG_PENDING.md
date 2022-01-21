### Improvements

- [cli] Download provider plugins from GitHub Releases
  [#8785](https://github.com/pulumi/pulumi/pull/8785)

- [sdk/dotnet] - Changed `Output<T>.ToString()` to return an informative message rather than just "Output`1[X]"
  [#8767](https://github.com/pulumi/pulumi/pull/8767)

- [cli] Add the concept of sequence numbers to the engine and resource provider interface.
  [#8631](https://github.com/pulumi/pulumi/pull/8631)

### Bug Fixes

- [sdk/python] - Prevent `ResourceOptions.merge` from promoting between the
  `.provider` and `.providers` fields. This is a subtle breaking change. This changes
  the general behavior of merging for `.provider` and `.providers`, as described in [#8796](https://github.com/pulumi/pulumi/issues/8796).
  [#8770](https://github.com/pulumi/pulumi/pull/8770)

- [codegen/nodejs] - Generate an install script that runs `pulumi plugin install` with
  the `--server` flag when necessary.
  [#8730](https://github.com/pulumi/pulumi/pull/8730)
