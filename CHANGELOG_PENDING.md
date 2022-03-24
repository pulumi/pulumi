### Improvements

- When a resource is aliased to an existing resource with a different URN, only store
  the alias of the existing resource in the statefile rather than storing all possible
  aliases.
  [#9288](https://github.com/pulumi/pulumi/pull/9288)

- Clear pending operations during `pulumi refresh` or `pulumi up -r`.
  [#8435](https://github.com/pulumi/pulumi/pull/8435)

### Bug Fixes

- [codegen/go] - Fix Go SDK function output to check for errors
  [pulumi-aws#1872](https://github.com/pulumi/pulumi-aws/issues/1872)
