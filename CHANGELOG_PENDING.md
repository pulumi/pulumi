### Features


### Improvements

- [sdk/go] Add helpers to convert raw Go maps and arrays to Pulumi `Map` and `Array` inputs.
  [#6337](https://github.com/pulumi/pulumi/pull/6337)

- [sdk/go] Return zero values instead of panicing in `Index` and `Elem` methods.
  [#6338](https://github.com/pulumi/pulumi/pull/6338)

- [cli] Add ability to download arm64 provider plugins
  [#6492](https://github.com/pulumi/pulumi/pull/6492)

- [build] Updating Pulumi to use Go 1.16
  [#6470](https://github.com/pulumi/pulumi/pull/6470)

- [build] Adding a Pulumi arm64 binary for use on new macOS hardware.  
  Please note that `pulumi watch` will not be supported on darwin/arm64 builds.
  [#6492](https://github.com/pulumi/pulumi/pull/6492)

### Bug Fixes

- [sdk/python] Fix mocks issue when passing a resource more than once.
  [#6479](https://github.com/pulumi/pulumi/pull/6479)
