### Improvements

- [dotnet] No longer roundtrips requests for the stack URN via the engine.
  [#9515](https://github.com/pulumi/pulumi/pull/9515)

### Bug Fixes

- [codegen/go] Enable obtaining resource outputs off a ResourceOutput.
  [#9513](https://github.com/pulumi/pulumi/pull/9513)

- [codegen/go] Ensure that "plain" generates shallowly plain types.
  [#9512](https://github.com/pulumi/pulumi/pull/9512)

- [codegen/nodejs] Fix enum naming when the enum name starts with `_`.
  [#9453](https://github.com/pulumi/pulumi/pull/9453)

- [cli] Empty passphrases environment variables are now treated as if the variable was not set.
  [#9490](https://github.com/pulumi/pulumi/pull/9490)

- [sdk/go] Fix awaits for outputs containing resources.
  [#9106](https://github.com/pulumi/pulumi/pull/9106)

- [cli] Decode YAML mappings with numeric keys during diff.
  [#9502](https://github.com/pulumi/pulumi/pull/9503)

- [cli] Fix an issue with explicit and default organization names in `pulumi new`
  [#9514](https://github.com/pulumi/pulumi/pull/9514)
