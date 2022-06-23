### Improvements

- [cli] The engine will now warn when a resource option is applied to a Component resource when that option will have no effect. This extends [#9863](https://github.com/pulumi/pulumi/pull/9863) which only warns for the `ignoreChanges` resource options.
  [#9921](https://github.com/pulumi/pulumi/pull/9921)

### Bug Fixes

- [auto/go] Fix passing of the color option.
  [#9940](https://github.com/pulumi/pulumi/pull/9940)