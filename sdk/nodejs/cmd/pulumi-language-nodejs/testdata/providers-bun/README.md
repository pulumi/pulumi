The SDKs used in the providers are copies of the SDKs from the tsnode/sdks snapshots, with the
following bun-specific modifications:

- `PulumiPlugin.yaml` sets `runtime: bun`
- The provider `package.json` adds `trustedDependencies` so bun runs the SDK's `postinstall` script
- The SDK `package.json` adds a `postinstall` script that compiles TypeScript to `bin/` and sets `main` to
  `bin/index.js`. In practice, published SDKs are distributed as compiled JavaScript. The `typescript` and `@types/node`
  packages are moved to `dependencies` (rather than `devDependencies`) so they are available during `postinstall`.
- The SDK `package.json` declares `@pulumi/pulumi` as a `peerDependency` rather than a regular dependency. This ensures
  bun uses the `@pulumi/pulumi` already installed at the provider level rather than fetching a separate copy from npm.
