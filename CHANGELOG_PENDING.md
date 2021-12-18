### Improvements

- [engine] - Interpret `pluginDownloadURL` as the provider host url when
  downloading plugins.
  [#8544](https://github.com/pulumi/pulumi/pull/8544)

### Bug Fixes

- [sdk/nodejs] - Fix `MockMonitor.readResource` to always call `mocks.newResource` with
  `custom: true` rather than crash.
  