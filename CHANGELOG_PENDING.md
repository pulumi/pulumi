### Improvements

- [cli] Using a decryptAll functionality when deserializing a deployment. This will allow
  decryption of secrets stored in the Pulumi Service backend to happen in bulk for
  performance increase
  [#8676](https://github.com/pulumi/pulumi/pull/8676)

### Bug Fixes

- [cli/engine] - Fix [#3982](https://github.com/pulumi/pulumi/issues/3982), a bug
  where the engine ignored the final line of stdout/stderr if it didn't terminate
  with a newline. [#8671](https://github.com/pulumi/pulumi/pull/8671)
