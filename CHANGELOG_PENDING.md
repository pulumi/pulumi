### Improvements

- [sdk/nodejs] Prevent Pulumi from overriding tsconfig.json options.
  [#7068](https://github.com/pulumi/pulumi/pull/7068)

### Bug Fixes

- [cli] - Properly parse Git remotes with periods or hyphens.
  [#7386](https://github.com/pulumi/pulumi/pull/7386)
  
- [codegen/python] - Recover good IDE completion experience over
  module imports that was compromised when introducing the lazy import
  optimization.
  [#7487](https://github.com/pulumi/pulumi/pull/7487)
