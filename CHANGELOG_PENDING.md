### Improvements
  
- [codegen] - Encrypt input args for secret properties.
  [#7128](https://github.com/pulumi/pulumi/pull/7128)

### Bug Fixes

- [CLI] Fix broken venv for Python projects started from templates
  [#6624](https://github.com/pulumi/pulumi/pull/6623)
  
- [cli] - Send plugin install output to stderr, so that it doesn't
  clutter up --json, automation API scenarios, and so on.
  [#7115](https://github.com/pulumi/pulumi/pull/7115)
