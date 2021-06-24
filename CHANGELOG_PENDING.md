**Please Note:** Release v3.5.0 failed in our build pipeline so will be rebuilt with a new tag of v3.5.1

### Improvements

- [cli] - Added support for passing custom paths that need
  to be watched by the `pulumi watch` command.
  [#7115](https://github.com/pulumi/pulumi/pull/7247)

- [auto/nodejs] - Fail early when multiple versions of `@pulumi/pulumi` are detected in nodejs inline programs.'
  [#7349](https://github.com/pulumi/pulumi/pull/7349)
  
- [sdk] - Add `replaceOnChanges` resource option.
  [#7226](https://github.com/pulumi/pulumi/pull/7226)

- [sdk/go] - Add preliminary support for unmarshaling plain arrays and maps of output values.
  [#7369](https://github.com/pulumi/pulumi/pull/7369)

- Initial support for resource methods (Node.js authoring, Python calling)
  [#7363](https://github.com/pulumi/pulumi/pull/7363)

### Bug Fixes

- [sdk/dotnet] - Fix swallowed nested exceptions with inline program, so they correctly bubble to the consumer.
  [#7323](https://github.com/pulumi/pulumi/pull/7323)
  
- [sdk/go] - Specify known when creating outputs for `construct`.
  [#7343](https://github.com/pulumi/pulumi/pull/7343)

- [cli] - Fix passphrase rotation.
  [#7347](https://github.com/pulumi/pulumi/pull/7347)
  
- [multilang/python] - Fix nested module generation.
  [#7353](https://github.com/pulumi/pulumi/pull/7353)

- [multilang/nodejs] - Fix a hang when an error is thrown within an apply in a remote component.
  [#7365](https://github.com/pulumi/pulumi/pull/7365)

- [codegen/python] - Include enum docstrings for python.
  [#7374](https://github.com/pulumi/pulumi/pull/7374)
