### Breaking


### Improvements

- [cli] Support full fidelity YAML round-tripping
  - Strip Byte-order Mark (BOM) from YAML configs during load. - [#6636](https://github.com/pulumi/pulumi/pull/6636)
  - Swap out YAML parser library - [#6642](https://github.com/pulumi/pulumi/pull/6642)

- [sdk/python] Ensure all async tasks are awaited prior to exit.
  [#6606](https://github.com/pulumi/pulumi/pull/6606)

### Bug Fixes

- [automation/python] Fix passing of additional environment variables.
  [#6639](https://github.com/pulumi/pulumi/pull/6639)
  
- [sdk/python] Make exceptions raised by calls to provider functions (e.g. data sources) catchable.
  [#6504](https://github.com/pulumi/pulumi/pull/6504)
