### Improvements

- [cli] - Add `--exclude-protected` flag to `pulumi destroy`
  [#8359](https://github.com/pulumi/pulumi/pull/8359)

### Bug Fixes

- [sdk/dotnet] - Fixes failing preview for programs that call data
  sources (`F.Invoke`) with unknown outputs
  [#8339](https://github.com/pulumi/pulumi/pull/8339)

- [programgen/go] - Don't change imported resource names.
  [#8353](https://github.com/pulumi/pulumi/pull/8353)
