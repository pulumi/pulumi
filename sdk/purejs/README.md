# purejs policy packs

`pulumi-language-purejs` runs policy packs whose `PulumiPolicy.yaml` says `runtime: purejs`. A purejs pack is one
self-contained JavaScript file. The host evaluates it with [goja](https://github.com/dop251/goja), a JavaScript
interpreter written in Go, so the CLI can enforce the pack without Node.js, npm or a dependency install.

The CLI needs no changes to use it: it already runs a pack of runtime `X` through the language plugin
`pulumi-language-X`. The host answers `InstallDependencies` with nothing to do, and `RunPlugin` loads the bundle
and serves it as the same `Analyzer` gRPC service a Node.js pack serves. One process serves each pack.

## Pack layout

```
my-pack/
├── PulumiPolicy.yaml
└── policies.js
```

```yaml
runtime: purejs
main: policies.js   # the default
version: 1.2.3      # wins over the version in the bundle
description: Optional; used when the bundle has none.
```

## Bundle contract (ABI version 1)

The bundle sets `globalThis.__pulumiPolicyPack`:

```js
globalThis.__pulumiPolicyPack = {
  abiVersion: 1,
  name: "my-pack",
  version: "1.2.3",
  enforcementLevel: "advisory",   // the pack's default
  policies: [
    {
      name: "s3-versioning",
      description: "S3 buckets must be versioned.",
      enforcementLevel: "mandatory",
      severity: "high",
      kind: "resource",           // or "stack"
      resourceTypes: ["aws:s3/bucket:Bucket"],
      configSchema: { properties: { exempt: { type: "array" } } },
      validate: function (input, ctx) {
        if (!input.props.versioning?.enabled) ctx.reportViolation("versioning is off");
      },
    },
  ],
};
```

Policies may also set `displayName`, `message`, `framework`, `tags`, `remediationSteps` and `url`. The pack may set
`displayName`, `description`, `readme`, `provider`, `repository`, `tags` and `initialConfig`.

What `validate` receives:

- **Resource policies:** `{type, name, urn, props, config}`, once per resource of a matching type.
- **Stack policies:** `{resources, config}`, once per stack. Each resource is
  `{type, name, urn, props, parent?, dependencies, propertyDependencies}`.

The `ctx` argument:

- `ctx.reportViolation(message, urn?)` reports a violation. `message` is a string or an object with `message` and
  optionally `urn`, `severity`, `enforcementLevel`, `name` or `description`.
- `ctx.notApplicable(reason)` says the policy doesn't apply.

Values in `props` are presented the way the Node.js policy SDK presents them: secrets are unwrapped, and resource
references become their ID or URN. Reading a value that is unknown during preview throws. The host then reports an
advisory "can't run policy … during preview" diagnostic, and keeps any violations reported before the read.

The bundle runs in a bare JavaScript runtime (ES2023 or so). The host adds `console`, `atob`, `btoa`, `URL` and the part of
`Buffer` that decodes and encodes base64 text. There is no `require`, filesystem, network or timers, and `validate`
must be synchronous. Remediation is not supported: a `remediate` policy reports its violations as mandatory.

## Building bundles

A bundle can be written by hand, or built from an existing `@pulumi/policy` pack. To build one, bundle the pack
(for example with esbuild) against a small shim of `@pulumi/policy` that records the `PolicyPack` it constructs as
`globalThis.__pulumiPolicyPack`. That tooling is not part of this repository yet.

## Development

```sh
cd sdk/purejs && make build test_fast   # builds ../../bin/pulumi-language-purejs
```

The module imports only the gRPC protocol (`sdk/proto/go`), goja and a YAML parser. It doesn't import the SDK's
other Go packages, which would bring in most of the SDK's dependencies and more than double the binary's size.
