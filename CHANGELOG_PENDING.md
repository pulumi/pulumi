### Improvements

### Bug Fixes

- Fix invalid resource type on `pulumi convert` to Go
  [#10670](https://github.com/pulumi/pulumi/pull/10670)

- [auto/nodejs] `onOutput` is now called incrementally as the
  underyling Pulumi process produces data, instead of being called
  once at the end of the process execution. This restores behavior
  that regressed since 3.39.0.
  [#10678](https://github.com/pulumi/pulumi/pull/10678)
