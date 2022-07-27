### Improvements

- [sdk/nodejs] Removed stack trace output for Typescript compilation errors
  [#10259](https://github.com/pulumi/pulumi/pull/10259)

### Bug Fixes

- [cli] Fix installation failures on Windows due to release artifacts shipped omitting a folder, `pulumi/*.exe` instead
  of `pulumi/bin/*.exe`.
  [#10264](https://github.com/pulumi/pulumi/pull/10264)
