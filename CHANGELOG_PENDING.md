### Improvements

- [sdk/nodejs] - Type optional input properties as `Input<T | undefined> | undefined` instead of
  `Input<T> | undefined`. This allows users to pass `Output<T | undefined>` for optional input
  properties.
  [#6323](https://github.com/pulumi/pulumi/pull/6323)

### Bug Fixes

