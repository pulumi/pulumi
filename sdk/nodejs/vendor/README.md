# Vendor

This directory contains vendored versions of [Typescript 3.8.3](https://github.com/microsoft/TypeScript/tree/v3.8.3) and [ts-node 7.0.1](https://github.com/TypeStrong/ts-node/tree/v7.0.1).

These are the default and minimum versions we support for these packages.

Historically these packages were direct dependencies of `@pulumi/pulumi`. To decouple the node SDK from the precise version of TypeScript, the packages are now declared as optional peer pependencies of `@pulumi/pulumi` and customers can pick the versions they want.

The reason we mark the peer dependencies as *optional* is to prevent package managers from automatically installing them. This avoids the situation where the package manger would install a more recent version of TypeScript without the user explictly opting in. Newer versions have stricter type checks, and can thus stop existing programs from running successfully.

When the peer dependencies are not present, we load the vendored versions of the modules.

## TypeScript

To vendor typescript:

```bash
cd sdks/nodejs/vendor
curl -L -o typescript-3.8.3.tgz https://registry.npmjs.org/typescript/-/typescript-3.8.3.tgz
tar xvf typescript-3.8.3.tgz
rsync package/LICENSE.txt package/CopyrightNotice.txt package/ThirdPartyNoticeText.txt package/lib/typescript.js typescript@3.8.3/
rsync package/lib/*.d.ts typescript@3.8.3/
rm -rf package
rm typescript-3.8.3.tgz
```

## ts-node

ts-node has its own dependencies that we need to include. Instead of copying another level of `node_modules` into our repository, we create a bundle using [esbuild](https://esbuild.github.io):

```bash
cd sdks/nodejs/vendor
curl -L -o ts-node-7.0.1.tgz https://registry.npmjs.org/ts-node/-/ts-node-7.0.1.tgz
tar xvf ts-node-7.0.1.tgz
cd package
npm install --omit=dev --no-package-lock --no-bin-links --ignore-scripts
cd ..
npx esbuild --bundle --platform=node --target=node18.0 --outdir=ts-node@7.0.1 --format=cjs package/dist/index.js
cp package/LICENSE ts-node@7.0.1
rm -rf package
rm ts-node-7.0.1.tgz
```
