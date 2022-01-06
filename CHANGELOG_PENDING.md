### Improvements

- [codegen/go] - Populate `PluginDownloadURL` in Resource and Invoke requests
  when `pluginDOwnloadURL` is provided by the generating schema.
  [#8690](https://github.com/pulumi/pulumi/pull/8690)

- [sdk/go] - Add `PluginDownloadURL` as a resource option.
  [#8555](https://github.com/pulumi/pulumi/pull/8555)

- [sdk/go] - Allow users to override enviromental variables for `GetCommandResults`.
  [#8610](https://github.com/pulumi/pulumi/pull/8610)

- [sdk/nodejs] Support using native ES modules as Pulumi scripts
  [#7764](https://github.com/pulumi/pulumi/pull/7764)

- [sdk/nodejs] Support a `nodeargs` option for passing `node` arguments to the Node language host
  [#8655](https://github.com/pulumi/pulumi/pull/8655)

### Bug Fixes

- [cli/engine] - Fix [#3982](https://github.com/pulumi/pulumi/issues/3982), a bug
  where the engine ignored the final line of stdout/stderr if it didn't terminate
  with a newline. 
  [#8671](https://github.com/pulumi/pulumi/pull/8671)
