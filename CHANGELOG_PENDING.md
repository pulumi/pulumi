### Improvements

- [provider/python]: Improved exception display. The traceback is now shorter and it always starts with user code.  
  [#10336](https://github.com/pulumi/pulumi/pull/10336)

### Bug Fixes

- [engine/backends]: Fix bug where File state backend failed to apply validation to stack names, resulting in a panic.
  [#10417](https://github.com/pulumi/pulumi/pull/10417)
