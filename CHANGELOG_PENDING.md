### Improvements

### Bug Fixes

- [sdk/go] - Normalize merge behavior for `ResourceOptions`, inline
  with other SDKs. See https://github.com/pulumi/pulumi/issues/8796 for more
  details.
  [#8882](https://github.com/pulumi/pulumi/pull/8882)

- [cli] - Encrypt resource `"id"`s if they are secret when serializing to
  state.
  [#8948](https://github.com/pulumi/pulumi/pull/8948)
