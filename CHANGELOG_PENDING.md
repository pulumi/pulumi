### Improvements

- [codegen/go] Chunk the `pulumiTypes.go` file to reduce max file size.
  [#10666](https://github.com/pulumi/pulumi/pull/10666)

### Bug Fixes

- Fix invalid resource type on `pulumi convert` to Go
  [#10670](https://github.com/pulumi/pulumi/pull/10670)

- [auto/nodejs] `onOutput` is now called incrementally as the
  underyling Pulumi process produces data, instead of being called
  once at the end of the process execution. This restores behavior
  that regressed since 3.39.0.
  [#10678](https://github.com/pulumi/pulumi/pull/10678)

- [sdk/nodejs] The `@pulumi/pulumi` package is now interoperable with ESModules.
  [#10622](https://github.com/pulumi/pulumi/pull/10622)
