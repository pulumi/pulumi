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

- [sdk/python] update protobuf library to v4 to speed up pulumi CLI dramatically on M1 machines
  [#10063](https://github.com/pulumi/pulumi/pull/10063)