### Improvements

- [cli/projects] - "runtime" in Pulumi.yaml can now specify a version. This is currently unused but will be used for language plugins distributed unbundled from the main pulumi binary.
  [#9244](https://github.com/pulumi/pulumi/pull/9244)

### Bug Fixes

- [codegen/go] - Fix Go SDK function output to check for errors
  [pulumi-aws#1872](https://github.com/pulumi/pulumi-aws/issues/1872)
