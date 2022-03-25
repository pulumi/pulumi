### Improvements

- Clear pending operations during `pulumi refresh` or `pulumi up -r`.
  [#8435](https://github.com/pulumi/pulumi/pull/8435)

- [cli] Warn users when there are pending operations but proceed with deployment
  [#9293](https://github.com/pulumi/pulumi/pull/9293)

### Bug Fixes

- [codegen/go] - Fix Go SDK function output to check for errors
  [pulumi-aws#1872](https://github.com/pulumi/pulumi-aws/issues/1872)
