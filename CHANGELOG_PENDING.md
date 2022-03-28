### Improvements

- Clear pending operations during `pulumi refresh` or `pulumi up -r`.
  [#8435](https://github.com/pulumi/pulumi/pull/8435)

### Bug Fixes

- [codegen/go] - Fix Go SDK function output to check for errors
  [pulumi-aws#1872](https://github.com/pulumi/pulumi-aws/issues/1872)

- [cli/engine] - Fix a panic due to `Check` returning nil while using update plans.
  [#9304](https://github.com/pulumi/pulumi/pull/9304)