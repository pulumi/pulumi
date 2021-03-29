### Breaking


### Improvements

- [cli] Strip Byte-order Mark (BOM) from YAML configs during load.
  [#6636](https://github.com/pulumi/pulumi/pull/6636)

### Bug Fixes

- [sdk/nodejs] Fix error propagation in registerResource and other resource methods.
  [#6644](https://github.com/pulumi/pulumi/pull/6644)

- [automation/python] Fix passing of additional environment variables.
  [#6639](https://github.com/pulumi/pulumi/pull/6639)
  
- [sdk/python] Make exceptions raised by calls to provider functions (e.g. data sources) catchable.
  [#6504](https://github.com/pulumi/pulumi/pull/6504)
  