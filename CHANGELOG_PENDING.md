### Improvements

- [codegen/dotnet] - Add C# extension `rootNamespace`, allowing the user to
  replace `Pulumi` as the default C# global namespace in generated programs.
  The `Company` and `Author` fields of the .csproj file are now driven by
  `schema.publisher`.
  [#8735](https://github.com/pulumi/pulumi/pull/8735)

- [cli] Download provider plugins from GitHub Releases
  [#8785](https://github.com/pulumi/pulumi/pull/8785)

- [sdk/dotnet] - Changed `Output<T>.ToString()` to return an informative message rather than just "Output`1[X]"
  [#8767](https://github.com/pulumi/pulumi/pull/8767)

- [cli] Add the concept of sequence numbers to the engine and resource provider interface.
  [#8631](https://github.com/pulumi/pulumi/pull/8631)

- [common] Allow names with hyphens.

- [cli] - Add support for overriding plugin download URLs.
  [#8798](https://github.com/pulumi/pulumi/pull/8798)

- [sdk/nodejs] - Support top-level default exports in ESM.
  [#8766](https://github.com/pulumi/pulumi/pull/8766)

### Bug Fixes

- [sdk/python] - Prevent `ResourceOptions.merge` from promoting between the
  `.provider` and `.providers` fields. This changes the general behavior of merging
  for `.provider` and `.providers`, as described in [#8796](https://github.com/pulumi/pulumi/issues/8796).
  Note that this is a breaking change in two ways:
    1. Passing a provider to a custom resource of the wrong package now
       produces a `ValueError`. In the past it would send to the provider, and
       generally crash the provider.
    2. Merging two `ResourceOptions` with `provider` set no longer hoists to `providers`.
       One `provider` will now take priority over the other. The new behavior reflects the
       common case for `ResourceOptions.merge`. To restore the old behavior, replace
       `ResourceOptions(provider=FooProvider).merge(ResourceOptions(provider=BarProvider))`
       with `ResourceOptions(providers=[FooProvider]).merge(ResourceOptions(providers=[BarProvider]))`.
  [#8770](https://github.com/pulumi/pulumi/pull/8770)

- [codegen/nodejs] - Generate an install script that runs `pulumi plugin install` with
  the `--server` flag when necessary.
  [#8730](https://github.com/pulumi/pulumi/pull/8730)

- [cli] The engine will no longer try to replace resources that are protected as that entails a delete.
  [#8810](https://github.com/pulumi/pulumi/pull/8810)

- [codegen/pcl] - Fix handling of function invokes without args
  [#8805](https://github.com/pulumi/pulumi/pull/8805)
