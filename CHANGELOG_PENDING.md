### Improvements

- [plugins] Plugin download urls now support GitHub as a first class url schema. For example "github://api.github.com/pulumiverse".
  [#9984](https://github.com/pulumi/pulumi/pull/9984)

- [nodejs] No longer roundtrips requests for the stack URN via the engine.
  [#9680](https://github.com/pulumi/pulumi/pull/9680)

### Bug Fixes

- [cli] `pulumi convert` supports provider packages without a version.
  [#9976](https://github.com/pulumi/pulumi/pull/9976)

- [cli] Revert changes to how --target works. This means that non-targeted resources do need enough valid inputs to pass Check.
  [#10024](https://github.com/pulumi/pulumi/pull/10024)