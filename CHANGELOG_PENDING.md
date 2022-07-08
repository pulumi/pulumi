### Improvements

- [cli] Display outputs during the very first preview.
  [#10031](https://github.com/pulumi/pulumi/pull/10031)

### Bug Fixes

- [cli] `pulumi convert` help text is wrong
  [#9892](https://github.com/pulumi/pulumi/issues/9892)

- [go/codegen] fix error assignment when creating a new resource in generated go code
  [#10049](https://github.com/pulumi/pulumi/pull/10049)

- [cli] `pulumi convert` generates incorrect input parameter names for C#
  [#10042](https://github.com/pulumi/pulumi/issues/10042)

- [engine] Un-parent child resource when a resource is deleted during a refresh.
  [#10073](https://github.com/pulumi/pulumi/pull/10073)
