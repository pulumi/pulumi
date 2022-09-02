### Improvements

- [cli] Display outputs last in diff view.
  [#10535](https://github.com/pulumi/pulumi/pull/10535)

- [sdk/python] Dropped support for Python 3.6.
  [#10529](https://github.com/pulumi/pulumi/pull/10529)

- [codegen/nodejs] Support lazy-loading Node modules.
  [#10538](https://github.com/pulumi/pulumi/pull/10538)

- [cli/backend] Gzip compress HTTPS payloads for `pulumi import` and secret decryption against
  the Pulumi Service backend.
  [#10558](https://github.com/pulumi/pulumi/pull/10558)

### Bug Fixes

- [cli/refresh] Do not panic when snapshot is `nil`.
  [#10593](https://github.com/pulumi/pulumi/pull/10593)
