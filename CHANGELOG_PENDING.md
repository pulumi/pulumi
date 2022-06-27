### Improvements

- [sdk/go] Added `PreviewDigest` for third party tools to be able to ingest the preview json
  [#9886](https://github.com/pulumi/pulumi/pull/9886)

- [cli] Do not require the `--yes` option if the `--skip-preview` option is set.
  [#9972](https://github.com/pulumi/pulumi/pull/9972)

### Bug Fixes

- [engine] Filter out non-targeted resources much earlier in the engine cycle.
  [#9960](https://github.com/pulumi/pulumi/pull/9960)
