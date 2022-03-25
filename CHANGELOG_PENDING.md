### Improvements

- Clear pending operations during `pulumi refresh` or `pulumi up -r`.
  [#8435](https://github.com/pulumi/pulumi/pull/8435)

- [cli] - Installing of language specific project dependencies is now managed by the language plugins, not the pulumi cli.
  [#9294](https://github.com/pulumi/pulumi/pull/9294)
### Bug Fixes

- [codegen/go] - Fix Go SDK function output to check for errors
  [pulumi-aws#1872](https://github.com/pulumi/pulumi-aws/issues/1872)
