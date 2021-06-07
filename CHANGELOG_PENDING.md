### Improvements

- [codegen] - Encrypt input args for secret properties.
  [#7128](https://github.com/pulumi/pulumi/pull/7128)

- [codegen] - Include properties with an underlying type of string on Go provider instances.

### Bug Fixes

- [cli] - Send plugin install output to stderr, so that it doesn't
  clutter up --json, automation API scenarios, and so on.
  [#7115](https://github.com/pulumi/pulumi/pull/7115)

- [auto/nodejs] - Emit warning instead of breaking on parsing JSON events for automation API.
  [#7162](https://github.com/pulumi/pulumi/pull/7162)

- [sdk/python] Improve performance of `Output.from_input` and `Output.all` on nested objects.
  [#7175](https://github.com/pulumi/pulumi/pull/7175)
