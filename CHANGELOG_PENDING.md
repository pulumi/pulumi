### Improvements

- [sdk/python] Improve error message when pulumi-python cannot find a main program.
  [#10617](https://github.com/pulumi/pulumi/pull/10617)

- [cli] provide info message to user if a pulumi program contains no resources
  [#10461](https://github.com/pulumi/pulumi/issues/10461)

### Bug Fixes

- [engine/plugins]: Revert change causing third party provider packages to prevent deployment commands (`up`, `preview`, ...)
  when used with the nodejs runtime. Reverts #10530.
  [#10650](https://github.com/pulumi/pulumi/pull/10650)
