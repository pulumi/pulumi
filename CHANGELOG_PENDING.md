### Improvements

- [build] - make lint returns an accurate status code
  [#7844](https://github.com/pulumi/pulumi/pull/7844)

- [codegen/python] - Add helper function forms `$fn_output` that
  accept `Input`s, return an `Output`, and wrap the underlying `$fn`
  call. This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for Python,
  making it easier to compose functions/datasources with Pulumi
  resources. [#7784](https://github.com/pulumi/pulumi/pull/7784)

- [codegen] - Add `replaceOnChange` to schema.
  [#7874](https://github.com/pulumi/pulumi/pull/7874)

- [cli/about] - Add comand for debug information
  [#7817](https://github.com/pulumi/pulumi/pull/7817)

- [codegen/schema] Add a `pulumi schema check` command to validate package schemas.
  [#7865](https://github.com/pulumi/pulumi/pull/7865)

### Bug Fixes
- [automation/go] Fix loading of stack settings/configs from yaml files.
  [#pulumi-kubernetes-operator/183](https://github.com/pulumi/pulumi-kubernetes-operator/issues/183)
