### Improvements

- [codegen] - Encrypt input args for secret properties.
  [#7128](https://github.com/pulumi/pulumi/pull/7128)

### Bug Fixes

- [cli] - Send plugin install output to stderr, so that it doesn't
  clutter up --json, automation API scenarios, and so on.
  [#7115](https://github.com/pulumi/pulumi/pull/7115)

- [auto/nodejs] - Emit warning instead of breaking on parsing JSON events for automation API.
  [#7162](https://github.com/pulumi/pulumi/pull/7162)

- [auto/python] - Add function overloads for stack creation and selection.
  [#7167](https://github.com/pulumi/pulumi/pull/7167)
