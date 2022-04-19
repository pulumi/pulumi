### Improvements

- [cli] Split invoke request protobufs, as monitors and providers take different arguments.
  [#9323](https://github.com/pulumi/pulumi/pull/9323)

- [providers] - gRPC providers can now support an Attach method for debugging. The engine will attach to providers listed in the PULUMI_DEBUG_PROVIDERS environment variable. This should be of the form "providerName:port,otherProvider:port".
  [#8979](https://github.com/pulumi/pulumi/pull/8979)

### Bug Fixes

- [cli/plugin] - Dynamic provider binaries will now be found even if pulumi/bin is not on $PATH.
  [#9396](https://github.com/pulumi/pulumi/pull/9396)