### Improvements
- [area/cli] - Implemented `state rename` command.
  [#9098](https://github.com/pulumi/pulumi/pull/9098)

- [cli/plugins] `pulumi plugin install` can now look up the latest version of plugins on GitHub releases.
  [#9012](https://github.com/pulumi/pulumi/pull/9012)

- [cli/backend] - `pulumi cancel` is now supported for the file state backend.
  [#9100](https://github.com/pulumi/pulumi/pull/9100)

- [cli/import] - The import command no longer errors if resource properties do not validate. Instead the
  engine warns about property issues returned by the provider but then continues with the import and codegen
  as best it can. This should result in more resources being imported to the pulumi state and being able to
  generate some code, at the cost that the generated code may not work as is in an update. Users will have to
  edit the code to successfully run.
  [#8922](https://github.com/pulumi/pulumi/pull/8922)
  
- [cli/import] - Code generation in `pulumi import` can now be disabled with the `--generate-code=false` flag.
  [#9141](https://github.com/pulumi/pulumi/pull/9141)

### Bug Fixes

- [sdk/python] - Fix build warnings. See
  [#9011](https://github.com/pulumi/pulumi/issues/9011) for more details.
  [#9139](https://github.com/pulumi/pulumi/pull/9139)

- [cli/backend] - Fixed an issue with non-atomicity when saving file state stacks.
  [#9122](https://github.com/pulumi/pulumi/pull/9122)

- [sdk/go] - Fixed an issue where the RetainOnDelete resource option is not applied. 
  [#9147](https://github.com/pulumi/pulumi/pull/9147)
