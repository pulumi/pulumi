### Improvements

### Bug Fixes

- [codegen/nodejs] Fix enum naming when the enum name starts with `_`.
  [#9453](https://github.com/pulumi/pulumi/pull/9453)

- [cli] Empty passphrases environment variables are now treated as if the variable was not set.
  [#9490](https://github.com/pulumi/pulumi/pull/9490)

- [sdk/go] Fix awaits for outputs containing resources.
  [#9106](https://github.com/pulumi/pulumi/pull/9106)

- [cli] Decode YAML mappings with numeric keys during diff.
  [#9502](https://github.com/pulumi/pulumi/pull/9503)
