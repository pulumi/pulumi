### Improvements

- [sdk/go] - Allow users to override enviromental variables for `GetCommandResults`.
  [#8610](https://github.com/pulumi/pulumi/pull/8610)

### Bug Fixes

- [cli/engine] - Fix [#3982](https://github.com/pulumi/pulumi/issues/3982), a bug
  where the engine ignored the final line of stdout/stderr if it didn't terminate
  with a newline. [#8671](https://github.com/pulumi/pulumi/pull/8671)
